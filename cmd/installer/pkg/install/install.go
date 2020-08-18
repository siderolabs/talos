// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/version"
)

// Options represents the set of options available for an install.
type Options struct {
	ConfigSource    string
	Disk            string
	Platform        string
	ExtraKernelArgs []string
	Bootloader      bool
	Upgrade         bool
	Force           bool
	Zero            bool
	Save            bool
}

// Install installs Talos.
func Install(p runtime.Platform, seq runtime.Sequence, opts *Options) (err error) {
	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamPlatform, p.Name())
	cmdline.Append(constants.KernelParamConfig, opts.ConfigSource)

	if err = cmdline.AppendAll(p.KernelArgs().Strings()); err != nil {
		return err
	}

	if err = cmdline.AppendAll(opts.ExtraKernelArgs); err != nil {
		return err
	}

	cmdline.AppendDefaults()

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

	Current string
	Next    string

	bootPartitionFound bool
}

// NewInstaller initializes and returns an Installer.
//
// nolint: gocyclo
func NewInstaller(cmdline *procfs.Cmdline, seq runtime.Sequence, opts *Options) (i *Installer, err error) {
	i = &Installer{
		cmdline:    cmdline,
		options:    opts,
		bootloader: &grub.Grub{},
	}

	var dev *probe.ProbedBlockDevice

	if dev, err = probe.GetDevWithFileSystemLabel(constants.BootPartitionLabel); err != nil {
		i.bootPartitionFound = false
	} else {
		i.bootPartitionFound = true
	}

	if seq == runtime.SequenceUpgrade && i.bootPartitionFound {
		if err = os.MkdirAll("/boot", 0o777); err != nil {
			return nil, err
		}

		if err = unix.Mount(dev.Path, "/boot", dev.SuperBlock.Type(), 0, ""); err != nil {
			return nil, fmt.Errorf("failed to mount /boot: %w", err)
		}
	}

	i.Current, i.Next, err = i.bootloader.Labels()
	if err != nil {
		return nil, err
	}

	label := i.Current

	if seq == runtime.SequenceUpgrade && i.bootPartitionFound {
		label = i.Next
	}

	i.manifest, err = NewManifest(label, seq, i.options)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation manifest: %w", err)
	}

	return i, nil
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
//
// nolint: gocyclo
func (i *Installer) Install(seq runtime.Sequence) (err error) {
	if seq != runtime.SequenceUpgrade || !i.bootPartitionFound {
		if i.options.Zero {
			if err = zero(i.manifest); err != nil {
				return fmt.Errorf("failed to wipe device(s): %w", err)
			}
		}

		// Partition and format the block device(s).
		if err = i.manifest.ExecuteManifest(); err != nil {
			return err
		}

		// Mount the partitions.

		mountpoints := mount.NewMountPoints()
		// look for mountpoints across all target devices
		for dev := range i.manifest.Targets {
			var mp *mount.Points

			mp, err = mount.SystemMountPointsForDevice(dev)
			if err != nil {
				return err
			}

			iter := mp.Iter()
			for iter.Next() {
				mountpoints.Set(iter.Key(), iter.Value())
			}
		}

		if err = mount.Mount(mountpoints); err != nil {
			return err
		}

		// nolint: errcheck
		defer mount.Unmount(mountpoints)
	}

	if seq == runtime.SequenceUpgrade && i.bootPartitionFound && i.options.Force {
		for dev, targets := range i.manifest.Targets {
			var bd *blockdevice.BlockDevice

			if bd, err = blockdevice.Open(dev); err != nil {
				return err
			}

			// nolint: errcheck
			defer bd.Close()

			var pt table.PartitionTable

			pt, err = bd.PartitionTable()
			if err != nil {
				return err
			}

			for _, target := range targets {
				for _, part := range pt.Partitions() {
					switch target.Label {
					case constants.BootPartitionLabel, constants.EphemeralPartitionLabel:
						target.PartitionName, err = util.PartPath(target.Device, int(part.No()))
						if err != nil {
							return err
						}
					}
				}

				if target.Label == constants.BootPartitionLabel {
					continue
				}

				if err = target.Format(); err != nil {
					return fmt.Errorf("failed to format device: %w", err)
				}
			}
		}
	}

	// Install the assets.

	for _, targets := range i.manifest.Targets {
		for _, target := range targets {
			switch target.Label {
			case constants.BootPartitionLabel:
				if err = i.bootloader.Prepare(target.Device); err != nil {
					return err
				}
			case constants.EphemeralPartitionLabel:
				continue
			}

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

	if seq != runtime.SequenceUpgrade || !i.bootPartitionFound {
		i.cmdline.Append("initrd", filepath.Join("/", i.Current, constants.InitramfsAsset))

		grubcfg := &grub.Cfg{
			Default: i.Current,
			Labels: []*grub.Label{
				{
					Root:   i.Current,
					Initrd: filepath.Join("/", i.Current, constants.InitramfsAsset),
					Kernel: filepath.Join("/", i.Current, constants.KernelAsset),
					Append: i.cmdline.String(),
				},
			},
		}

		if err = i.bootloader.Install("", grubcfg, seq, i.bootPartitionFound); err != nil {
			return err
		}
	} else {
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
				{
					Root:   i.Current,
					Initrd: filepath.Join("/", i.Current, constants.InitramfsAsset),
					Kernel: filepath.Join("/", i.Current, constants.KernelAsset),
					Append: procfs.ProcCmdline().String(),
				},
			},
		}

		if err = i.bootloader.Install(i.Current, grubcfg, seq, i.bootPartitionFound); err != nil {
			return err
		}
	}

	if i.options.Save {
		u, err := url.Parse(i.options.ConfigSource)
		if err != nil {
			return err
		}

		if u.Scheme != "file" {
			return fmt.Errorf("file:// scheme must be used with the save option, have %s", u.Scheme)
		}

		src, err := os.Open(u.Path)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer src.Close()

		dst, err := os.OpenFile(constants.ConfigPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer dst.Close()

		_, err = io.Copy(dst, src)
		if err != nil {
			return err
		}
	}

	return nil
}

func zero(manifest *Manifest) (err error) {
	var zero *os.File

	if zero, err = os.Open("/dev/zero"); err != nil {
		return err
	}

	defer zero.Close() //nolint: errcheck

	for dev := range manifest.Targets {
		if err = func(dev string) error {
			var f *os.File

			if f, err = os.OpenFile(dev, os.O_RDWR, os.ModeDevice); err != nil {
				return err
			}

			defer f.Close() //nolint: errcheck

			var size uint64

			if _, _, ret := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKGETSIZE64, uintptr(unsafe.Pointer(&size))); ret != 0 {
				return fmt.Errorf("failed to got block device size: %v", ret)
			}

			if _, err = io.CopyN(f, zero, int64(size)); err != nil {
				return err
			}

			return f.Close()
		}(dev); err != nil {
			return err
		}
	}

	return zero.Close()
}
