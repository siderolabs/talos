// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/board"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	bootloaderoptions "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
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
	Platform          string
	Arch              string
	Board             string
	ExtraKernelArgs   []string
	Upgrade           bool
	Force             bool
	Zero              bool
	LegacyBIOSSupport bool
	MetaValues        MetaValues

	// Options specific for the image creation mode.
	ImageSecureboot bool
	Version         string
	BootAssets      bootloaderoptions.BootAssets
	Printf          func(string, ...any)
	MountPrefix     string
}

// Mode is the install mode.
type Mode int

const (
	// ModeInstall is the install mode.
	ModeInstall Mode = iota
	// ModeUpgrade is the upgrade mode.
	ModeUpgrade
	// ModeImage is the image creation mode.
	ModeImage
)

// IsImage returns true if the mode is image creation.
func (m Mode) IsImage() bool {
	return m == ModeImage
}

// Install installs Talos.
func Install(ctx context.Context, p runtime.Platform, mode Mode, opts *Options) (err error) {
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

	if opts.Board != constants.BoardNone {
		// board 'rpi_4' was removed in Talos 1.5 in favor of `rpi_generic`
		if opts.Board == "rpi_4" {
			opts.Board = constants.BoardRPiGeneric
		}

		var b runtime.Board

		b, err = board.NewBoard(opts.Board)
		if err != nil {
			return err
		}

		cmdline.Append(constants.KernelParamBoard, b.Name())

		cmdline.SetAll(b.KernelArgs().Strings())
	}

	if err = cmdline.AppendAll(
		opts.ExtraKernelArgs,
		procfs.WithOverwriteArgs("console"),
		procfs.WithOverwriteArgs(constants.KernelParamPlatform),
		procfs.WithDeleteNegatedArgs(),
	); err != nil {
		return err
	}

	i, err := NewInstaller(ctx, cmdline, mode, opts)
	if err != nil {
		return err
	}

	if err = i.Install(ctx, mode); err != nil {
		return err
	}

	i.options.Printf("installation of %s complete", version.Tag)

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
func NewInstaller(ctx context.Context, cmdline *procfs.Cmdline, mode Mode, opts *Options) (i *Installer, err error) {
	i = &Installer{
		cmdline: cmdline,
		options: opts,
	}

	if i.options.Version == "" {
		i.options.Version = version.Tag
	}

	if i.options.Printf == nil {
		i.options.Printf = log.Printf
	}

	if !i.options.Zero {
		i.bootloader, err = bootloader.Probe(ctx, i.options.Disk)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to probe bootloader: %w", err)
		}
	}

	i.options.BootAssets.FillDefaults(opts.Arch)

	bootLoaderPresent := i.bootloader != nil
	if !bootLoaderPresent {
		if mode.IsImage() {
			// on image creation, use the bootloader based on options
			i.bootloader = bootloader.New(opts.ImageSecureboot, opts.Version)
		} else {
			// on install/upgrade perform automatic detection
			i.bootloader = bootloader.NewAuto()
		}
	}

	i.manifest, err = NewManifest(mode, i.bootloader.UEFIBoot(), bootLoaderPresent, i.options)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation manifest: %w", err)
	}

	return i, nil
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
//
//nolint:gocyclo,cyclop
func (i *Installer) Install(ctx context.Context, mode Mode) (err error) {
	errataBTF()

	if mode == ModeUpgrade {
		if err = i.errataNetIfnames(); err != nil {
			return err
		}
	}

	if err = i.runPreflightChecks(mode); err != nil {
		return err
	}

	if err = i.installExtensions(); err != nil {
		return err
	}

	if err = i.manifest.Execute(); err != nil {
		return err
	}

	// Mount the partitions.
	mountpoints := mount.NewMountPoints()

	var bootLabels []string

	if i.bootloader.UEFIBoot() {
		bootLabels = []string{constants.EFIPartitionLabel}
	} else {
		bootLabels = []string{constants.BootPartitionLabel, constants.EFIPartitionLabel}
	}

	for _, label := range bootLabels {
		if err = func() error {
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

			bd, err = retryBlockdeviceOpen(device)
			if err != nil {
				return err
			}

			defer bd.Close() //nolint:errcheck

			var mountpoint *mount.Point

			mountpoint, err = mount.SystemMountPointForLabel(ctx, bd, label, mount.WithPrefix(i.options.MountPrefix))
			if err != nil {
				return err
			}

			mountpoints.Set(label, mountpoint)

			return nil
		}(); err != nil {
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

	// Install the bootloader.
	if err = i.bootloader.Install(bootloaderoptions.InstallOptions{
		BootDisk:    i.options.Disk,
		Arch:        i.options.Arch,
		Cmdline:     i.cmdline.String(),
		Version:     i.options.Version,
		ImageMode:   mode.IsImage(),
		MountPrefix: i.options.MountPrefix,
		BootAssets:  i.options.BootAssets,
		Printf:      i.options.Printf,
	}); err != nil {
		return err
	}

	if i.options.Board != constants.BoardNone {
		var b runtime.Board

		b, err = board.NewBoard(i.options.Board)
		if err != nil {
			return err
		}

		i.options.Printf("installing U-Boot for %q", b.Name())

		if err = b.Install(runtime.BoardInstallOptions{
			InstallDisk:     i.options.Disk,
			MountPrefix:     i.options.MountPrefix,
			UBootPath:       i.options.BootAssets.UBootPath,
			DTBPath:         i.options.BootAssets.DTBPath,
			RPiFirmwarePath: i.options.BootAssets.RPiFirmwarePath,
			Printf:          i.options.Printf,
		}); err != nil {
			return err
		}
	}

	if mode == ModeUpgrade || len(i.options.MetaValues.values) > 0 {
		var (
			metaState         *meta.Meta
			metaPartitionName string
		)

		for _, targets := range i.manifest.Targets {
			for _, target := range targets {
				if target.Label == constants.MetaPartitionLabel {
					metaPartitionName = target.PartitionName

					break
				}
			}

			if metaPartitionName != "" {
				break
			}
		}

		if metaPartitionName == "" {
			return errors.New("failed to detect META partition")
		}

		if metaState, err = meta.New(context.Background(), nil, meta.WithPrinter(i.options.Printf), meta.WithFixedPath(metaPartitionName)); err != nil {
			return err
		}

		var ok bool

		if mode == ModeUpgrade {
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

func (i *Installer) runPreflightChecks(mode Mode) error {
	if mode != ModeUpgrade {
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

func retryBlockdeviceOpen(device string) (*blockdevice.BlockDevice, error) {
	var bd *blockdevice.BlockDevice

	err := retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		var openErr error

		bd, openErr = blockdevice.Open(device)
		if openErr != nil && os.IsNotExist(openErr) {
			return retry.ExpectedError(openErr)
		}

		return openErr
	})

	return bd, err
}
