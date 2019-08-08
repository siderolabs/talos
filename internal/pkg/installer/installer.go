/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package installer

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/installer/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/pkg/installer/manifest"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/talos-systems/talos/pkg/version"
	"golang.org/x/sys/unix"
)

const (
	// DefaultSizeBootDevice is the default size of the boot partition.
	// TODO(andrewrynhard): We should inspect the sizes of the artifacts and dynamically set the boot partition's size.
	DefaultSizeBootDevice = 512 * 1000 * 1000
)

var (
	// DefaultURLBase is the base URL for all default artifacts.
	// TODO(andrewrynhard): We need to setup infrastructure for publishing artifacts and not depend on GitHub.
	DefaultURLBase = "https://github.com/talos-systems/talos/releases/download/" + version.Tag

	// DefaultKernelURL is the URL to the kernel.
	DefaultKernelURL = DefaultURLBase + "/vmlinuz"

	// DefaultInitramfsURL is the URL to the initramfs.
	DefaultInitramfsURL = DefaultURLBase + "/initramfs.xz"
)

// Installer represents the installer logic. It serves as the entrypoint to all
// installation methods.
type Installer struct {
	cmdline  *kernel.Cmdline
	data     *userdata.UserData
	manifest *manifest.Manifest
}

// NewInstaller initializes and returns an Installer.
func NewInstaller(cmdline *kernel.Cmdline, data *userdata.UserData) *Installer {
	i := &Installer{
		cmdline: cmdline,
		data:    data,
	}

	i.manifest = manifest.NewManifest(data)

	return i
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
// nolint: gocyclo
func (i *Installer) Install() (err error) {
	if i.data.Install == nil {
		return nil
	}

	// Verify that the target device(s) can satisify the requested options.

	if err = VerifyBootDevice(i.data); err != nil {
		return errors.Wrap(err, "failed to prepare boot device")
	}

	if err = VerifyDataDevice(i.data); err != nil {
		return errors.Wrap(err, "failed to prepare data device")
	}

	if i.data.Install.Wipe {
		if err = wipe(i.manifest); err != nil {
			return errors.Wrap(err, "failed to wipe device(s)")
		}
	}

	// Partition and format the block device(s).

	if err = i.manifest.ExecuteManifest(i.data, i.manifest); err != nil {
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
				// Install the bootloader.
				if err = syslinux.Prepare(target.Device); err != nil {
					return err
				}
			case constants.DataPartitionLabel:
				continue
			}

			// Handle the download and extraction of assets.
			if err = target.Save(); err != nil {
				return err
			}
		}
	}

	// Install the bootloader.

	if i.data.Install.Boot == nil {
		return nil
	}

	syslinuxcfg := &syslinux.Cfg{
		Default: "default",
		Labels: []*syslinux.Label{
			{
				Root:   "default",
				Kernel: filepath.Join("/", "default", filepath.Base(i.data.Install.Boot.Kernel)),
				Initrd: filepath.Join("/", "default", filepath.Base(i.data.Install.Boot.Initramfs)),
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

func wipe(manifest *manifest.Manifest) (err error) {
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
			return errors.Errorf("failed to got block device size: %v", ret)
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
