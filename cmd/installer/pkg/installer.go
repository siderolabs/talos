// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/cmd/installer/pkg/bootloader/syslinux"
	"github.com/talos-systems/talos/cmd/installer/pkg/manifest"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

// Installer represents the installer logic. It serves as the entrypoint to all
// installation methods.
type Installer struct {
	cmdline  *procfs.Cmdline
	install  machine.Install
	manifest *manifest.Manifest
	Current  string
	Next     string

	bootPartitionFound bool
}

// NewInstaller initializes and returns an Installer.
//
// nolint: gocyclo
func NewInstaller(cmdline *procfs.Cmdline, sequence runtime.Sequence, install machine.Install) (i *Installer, err error) {
	i = &Installer{
		cmdline: cmdline,
		install: install,
	}

	var dev *probe.ProbedBlockDevice

	if dev, err = probe.GetDevWithFileSystemLabel(constants.BootPartitionLabel); err != nil {
		i.bootPartitionFound = false

		log.Printf("WARNING: failed to find %s: %v", constants.BootPartitionLabel, err)
	} else {
		i.bootPartitionFound = true
	}

	if sequence == runtime.Upgrade && i.bootPartitionFound {
		if err = os.MkdirAll("/boot", 0777); err != nil {
			return nil, err
		}

		if err = unix.Mount(dev.Path, "/boot", dev.SuperBlock.Type(), 0, ""); err != nil {
			return nil, fmt.Errorf("failed to mount /boot: %w", err)
		}
	}

	i.Current, i.Next, err = syslinux.Labels()
	if err != nil {
		return nil, err
	}

	label := i.Current

	if sequence == runtime.Upgrade && i.bootPartitionFound {
		label = i.Next
	}

	i.manifest, err = manifest.NewManifest(label, sequence, install)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation manifest: %w", err)
	}

	return i, nil
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
//
// nolint: gocyclo
func (i *Installer) Install(sequence runtime.Sequence) (err error) {
	if sequence != runtime.Upgrade || !i.bootPartitionFound {
		if i.install.Zero() {
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

			mp, err = owned.MountPointsForDevice(dev)
			if err != nil {
				return err
			}

			iter := mp.Iter()
			for iter.Next() {
				mountpoints.Set(iter.Key(), iter.Value())
			}
		}

		m := manager.NewManager(mountpoints)
		if err = m.MountAll(); err != nil {
			return err
		}

		// nolint: errcheck
		defer m.UnmountAll()
	}

	if sequence == runtime.Upgrade && i.bootPartitionFound && i.install.Force() {
		for dev, targets := range i.manifest.Targets {
			var bd *blockdevice.BlockDevice

			if bd, err = blockdevice.Open(dev); err != nil {
				return err
			}

			// nolint: errcheck
			defer bd.Close()

			var pt table.PartitionTable

			pt, err = bd.PartitionTable(true)
			if err != nil {
				return err
			}

			for _, target := range targets {
				for _, part := range pt.Partitions() {
					switch target.Label {
					case constants.BootPartitionLabel, constants.EphemeralPartitionLabel:
						target.PartitionName = util.PartPath(target.Device, int(part.No()))
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
				if err = syslinux.Prepare(target.Device); err != nil {
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

	if !i.install.WithBootloader() {
		return nil
	}

	if sequence != runtime.Upgrade || !i.bootPartitionFound {
		i.cmdline.Append("initrd", filepath.Join("/", i.Current, constants.InitramfsAsset))

		syslinuxcfg := &syslinux.Cfg{
			Default: i.Current,
			Labels: []*syslinux.Label{
				{
					Root:   i.Current,
					Initrd: filepath.Join("/", i.Current, constants.InitramfsAsset),
					Kernel: filepath.Join("/", i.Current, constants.KernelAsset),
					Append: i.cmdline.String(),
				},
			},
		}

		if err = syslinux.Install("", syslinuxcfg, sequence, i.bootPartitionFound); err != nil {
			return err
		}
	} else {
		i.cmdline.Append("initrd", filepath.Join("/", i.Next, constants.InitramfsAsset))

		syslinuxcfg := &syslinux.Cfg{
			Default: i.Next,
			Labels: []*syslinux.Label{
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

		if err = syslinux.Install(i.Current, syslinuxcfg, sequence, i.bootPartitionFound); err != nil {
			return err
		}
	}

	return nil
}

func zero(manifest *manifest.Manifest) (err error) {
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
