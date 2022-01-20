// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/kernel"
	"github.com/talos-systems/talos/pkg/version"
)

// Options represents the set of options available for an install.
type Options struct {
	ConfigSource      string
	Disk              string
	Platform          string
	Arch              string
	Board             string
	ExtraKernelArgs   []string
	Bootloader        bool
	Upgrade           bool
	Force             bool
	Zero              bool
	LegacyBIOSSupport bool
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

	if err = cmdline.AppendAll(opts.ExtraKernelArgs, procfs.WithOverwriteArgs("console")); err != nil {
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

	bootPartitionFound bool

	Current string
	Next    string
}

// NewInstaller initializes and returns an Installer.
func NewInstaller(cmdline *procfs.Cmdline, seq runtime.Sequence, opts *Options) (i *Installer, err error) {
	i = &Installer{
		cmdline: cmdline,
		options: opts,
		bootloader: &grub.Grub{
			BootDisk: opts.Disk,
			Arch:     opts.Arch,
		},
	}

	if err = i.probeBootPartition(); err != nil {
		return nil, err
	}

	i.manifest, err = NewManifest(i.Next, seq, i.bootPartitionFound, i.options)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation manifest: %w", err)
	}

	return i, nil
}

// Verify existence of boot partition.
func (i *Installer) probeBootPartition() error {
	// there's no reason to discover boot partition if the disk is about to be wiped
	if !i.options.Zero {
		dev, err := blockdevice.Open(i.options.Disk)
		if err != nil {
			i.bootPartitionFound = false

			return err
		}

		defer dev.Close() //nolint:errcheck

		if part, err := dev.GetPartition(constants.BootPartitionLabel); err != nil {
			i.bootPartitionFound = false
		} else {
			i.bootPartitionFound = true

			// mount the boot partition temporarily to find the bootloader labels
			mountpoints := mount.NewMountPoints()

			partPath, err := part.Path()
			if err != nil {
				return err
			}

			fsType, err := part.Filesystem()
			if err != nil {
				return err
			}

			mountpoint := mount.NewMountPoint(partPath, constants.BootMountPoint, fsType, unix.MS_NOATIME|unix.MS_RDONLY, "")
			mountpoints.Set(constants.BootPartitionLabel, mountpoint)

			if err := mount.Mount(mountpoints); err != nil {
				log.Printf("warning: failed to mount boot partition %q: %s", partPath, err)
			} else {
				defer mount.Unmount(mountpoints) //nolint:errcheck
			}
		}
	}

	var err error

	// anyways run the Labels() to get the defaults initialized
	i.Current, i.Next, err = i.bootloader.Labels()

	return err
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
//
//nolint:gocyclo,cyclop
func (i *Installer) Install(seq runtime.Sequence) (err error) {
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

	if !i.options.Bootloader {
		return nil
	}

	i.cmdline.Append("initrd", filepath.Join("/", i.Next, constants.InitramfsAsset))

	grubcfg := &grub.Cfg{
		Default: i.Next,
		Labels: []*grub.Label{
			{
				Root:   i.Next,
				Initrd: filepath.Join("/", i.Next, constants.InitramfsAsset),
				Kernel: filepath.Join("/", i.Next, constants.KernelAsset),
				Append: i.cmdline.String(),
			},
		},
	}

	if i.Current != "" {
		grubcfg.Fallback = i.Current

		grubcfg.Labels = append(grubcfg.Labels, &grub.Label{
			Root:   i.Current,
			Initrd: filepath.Join("/", i.Current, constants.InitramfsAsset),
			Kernel: filepath.Join("/", i.Current, constants.KernelAsset),
			Append: procfs.ProcCmdline().String(),
		})
	}

	if err = i.bootloader.Install(i.Current, grubcfg, seq); err != nil {
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

	if seq == runtime.SequenceUpgrade {
		var meta *bootloader.Meta

		if meta, err = bootloader.NewMeta(); err != nil {
			return err
		}

		//nolint:errcheck
		defer meta.Close()

		if ok := meta.LegacyADV.SetTag(adv.Upgrade, i.Current); !ok {
			return fmt.Errorf("failed to set upgrade tag: %q", i.Current)
		}

		if err = meta.Write(); err != nil {
			return err
		}
	}

	return nil
}
