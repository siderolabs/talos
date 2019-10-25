// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/installer/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/pkg/installer/manifest"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

// Installer represents the installer logic. It serves as the entrypoint to all
// installation methods.
type Installer struct {
	cmdline  *kernel.Cmdline
	install  machine.Install
	manifest *manifest.Manifest
}

// NewInstaller initializes and returns an Installer.
func NewInstaller(cmdline *kernel.Cmdline, install machine.Install) (i *Installer, err error) {
	i = &Installer{
		cmdline: cmdline,
		install: install,
	}

	i.manifest, err = manifest.NewManifest(install)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation manifest: %w", err)
	}

	return i, nil
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
// nolint: gocyclo
func (i *Installer) Install() (err error) {
	if i.install.Zero() {
		if err = zero(i.manifest); err != nil {
			return fmt.Errorf("failed to wipe device(s): %w", err)
		}
	}

	// Partition and format the block device(s).

	if err = i.manifest.ExecuteManifest(i.manifest); err != nil {
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

	syslinuxcfg := &syslinux.Cfg{
		Default: "default",
		Labels: []*syslinux.Label{
			{
				Root:   "default",
				Initrd: filepath.Join("/", "default", constants.InitramfsAsset),
				Kernel: filepath.Join("/", "default", constants.KernelAsset),
				Append: i.cmdline.String(),
			},
		},
	}

	if err = syslinux.Install(filepath.Join(constants.BootMountPoint), syslinuxcfg); err != nil {
		return err
	}

	if err = ioutil.WriteFile(filepath.Join(constants.BootMountPoint, "installed"), []byte{}, 0400); err != nil {
		return err
	}

	return nil
}

func zero(manifest *manifest.Manifest) (err error) {
	var zero *os.File

	if zero, err = os.Open("/dev/zero"); err != nil {
		return err
	}

	for dev := range manifest.Targets {
		var f *os.File

		if f, err = os.OpenFile(dev, os.O_RDWR, os.ModeDevice); err != nil {
			return err
		}

		var size uint64

		if _, _, ret := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKGETSIZE64, uintptr(unsafe.Pointer(&size))); ret != 0 {
			return fmt.Errorf("failed to got block device size: %v", ret)
		}

		if _, err = io.CopyN(f, zero, int64(size)); err != nil {
			return err
		}

		if err = f.Close(); err != nil {
			return err
		}
	}

	if err = zero.Close(); err != nil {
		return err
	}

	return nil
}
