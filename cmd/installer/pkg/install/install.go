// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"context"
	"fmt"
	"log"

	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/board"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/version"
)

// Options represents the set of options available for an install.
type Options struct {
	ConfigSource      string
	Disk              string
	DiskSize          int
	Platform          string
	Arch              string
	Board             string
	ExtraKernelArgs   []string
	Upgrade           bool
	Force             bool
	Zero              bool
	LegacyBIOSSupport bool
	MetaValues        MetaValues
}

// Install installs Talos.
func Install(p runtime.Platform, seq runtime.Sequence, opts *Options) (err error) {
	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamPlatform, p.Name())

	if opts.ConfigSource != "" {
		cmdline.Append(constants.KernelParamConfig, opts.ConfigSource)
	}

	cmdline.SetAll(p.KernelArgs().Strings())

	// first defaults, then extra kernel args to allow extra kernel args to override defaults
	if err = cmdline.AppendAll(kernel.DefaultArgs); err != nil {
		return err
	}

	if err = cmdline.AppendAll(
		opts.ExtraKernelArgs,
		procfs.WithOverwriteArgs("console"),
		procfs.WithOverwriteArgs(constants.KernelParamPlatform),
	); err != nil {
		return err
	}

	i, err := NewInstaller(cmdline, seq, opts)
	if err != nil {
		return err
	}

	if err = i.Install(seq); err != nil {
		return err
	}

	log.Printf("installation of %s complete", version.Tag)

	return nil
}

// Installer represents the installer logic. It serves as the entrypoint to all
// installation methods.
type Installer struct {
	cmdline    *procfs.Cmdline
	options    *Options
	manifest   *Manifest
	bootloader bootloader.Bootloader
}

// NewInstaller initializes and returns an Installer.
func NewInstaller(cmdline *procfs.Cmdline, seq runtime.Sequence, opts *Options) (i *Installer, err error) {
	i = &Installer{
		cmdline: cmdline,
		options: opts,
	}

	i.bootloader, err = bootloader.Probe(i.options.Zero)
	if err != nil {
		return nil, fmt.Errorf("failed to probe bootloader: %w", err)
	}

	var bootLoaderPresent bool

	// we have a bootloader, so we can flip it
	if i.bootloader.Installed() {
		bootLoaderPresent = true

		if err = i.bootloader.Flip(); err != nil {
			return nil, fmt.Errorf("failed to flip bootloader: %w", err)
		}
	}

	bootLabel := i.bootloader.NextLabel()

	i.manifest, err = NewManifest(bootLabel, seq, bootLoaderPresent, i.options)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation manifest: %w", err)
	}

	return i, nil
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
//
//nolint:gocyclo,cyclop
func (i *Installer) Install(seq runtime.Sequence) (err error) {
	errataBTF()

	if seq == runtime.SequenceUpgrade {
		if err = i.errataNetIfnames(); err != nil {
			return err
		}
	}

	if err = i.runPreflightChecks(seq); err != nil {
		return err
	}

	if err = i.installExtensions(); err != nil {
		return err
	}

	if i.options.Board != constants.BoardNone {
		var b runtime.Board

		b, err = board.NewBoard(i.options.Board)
		if err != nil {
			return err
		}

		i.cmdline.Append(constants.KernelParamBoard, b.Name())

		i.cmdline.SetAll(b.KernelArgs().Strings())
	}

	if err = i.manifest.Execute(); err != nil {
		return err
	}

	// Mount the partitions.
	mountpoints := mount.NewMountPoints()

	for _, label := range []string{constants.BootPartitionLabel, constants.EFIPartitionLabel} {
		err = func() error {
			var device string
			// searching targets for the device to be used
		OuterLoop:
			for dev, targets := range i.manifest.Targets {
				for _, target := range targets {
					if target.Label == label {
						device = dev

						break OuterLoop
					}
				}
			}

			if device == "" {
				return fmt.Errorf("failed to detect %s target device", label)
			}

			var bd *blockdevice.BlockDevice

			bd, err = blockdevice.Open(device)
			if err != nil {
				return err
			}

			defer bd.Close() //nolint:errcheck

			var mountpoint *mount.Point

			mountpoint, err = mount.SystemMountPointForLabel(bd, label)
			if err != nil {
				return err
			}

			mountpoints.Set(label, mountpoint)

			return nil
		}()

		if err != nil {
			return err
		}
	}

	if err = mount.Mount(mountpoints); err != nil {
		return err
	}

	defer func() {
		e := mount.Unmount(mountpoints)
		if e != nil {
			log.Printf("failed to unmount: %v", e)
		}
	}()

	// Install the assets.
	for _, targets := range i.manifest.Targets {
		for _, target := range targets {
			// Handle the download and extraction of assets.
			if err = target.Save(); err != nil {
				return err
			}
		}
	}

	// Install the bootloader.
	if err = i.bootloader.Install(i.options.Disk, i.options.Arch, i.cmdline.String()); err != nil {
		return err
	}

	if i.options.Board != constants.BoardNone {
		var b runtime.Board

		b, err = board.NewBoard(i.options.Board)
		if err != nil {
			return err
		}

		log.Printf("installing U-Boot for %q", b.Name())

		if err = b.Install(i.options.Disk); err != nil {
			return err
		}
	}

	if seq == runtime.SequenceUpgrade || len(i.options.MetaValues.values) > 0 {
		var metaState *meta.Meta

		if metaState, err = meta.New(context.Background(), nil); err != nil {
			return err
		}

		var ok bool

		if seq == runtime.SequenceUpgrade {
			if ok, err = metaState.SetTag(context.Background(), meta.Upgrade, i.bootloader.PreviousLabel()); !ok || err != nil {
				return fmt.Errorf("failed to set upgrade tag: %q", i.bootloader.PreviousLabel())
			}
		}

		for _, v := range i.options.MetaValues.values {
			if ok, err = metaState.SetTag(context.Background(), v.Key, v.Value); !ok || err != nil {
				return fmt.Errorf("failed to set meta tag: %q -> %q", v.Key, v.Value)
			}
		}

		if err = metaState.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func (i *Installer) runPreflightChecks(seq runtime.Sequence) error {
	if seq != runtime.SequenceUpgrade {
		// pre-flight checks only apply to upgrades
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checks, err := NewPreflightChecks(ctx)
	if err != nil {
		return fmt.Errorf("error initializing pre-flight checks: %w", err)
	}

	defer checks.Close() //nolint:errcheck

	return checks.Run(ctx)
}
