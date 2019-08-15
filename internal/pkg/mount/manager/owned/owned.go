/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package owned

import (
	"log"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/constants"
	"golang.org/x/sys/unix"
)

// MountPointsForDevice returns the mountpoints required to boot the system.
// This function is called exclusively during installations ( both image
// creation and bare metall installs ). This is why we want to look up
// device by specified disk as well as why we don't want to grow any
// filesystems.
func MountPointsForDevice(devpath string) (mountpoints *mount.Points, err error) {
	mountpoints = mount.NewMountPoints()
	for _, name := range []string{constants.EphemeralPartitionLabel, constants.BootPartitionLabel} {
		var target string
		switch name {
		case constants.EphemeralPartitionLabel:
			target = constants.EphemeralMountPoint
		case constants.BootPartitionLabel:
			target = constants.BootMountPoint
		}

		var dev *probe.ProbedBlockDevice
		if dev, err = probe.DevForFileSystemLabel(devpath, name); err != nil {
			if name == constants.BootPartitionLabel {
				// A bootloader is not always required.
				log.Println("WARNING: no ESP partition was found")
				continue
			}
			return nil, errors.Errorf("probe device for filesystem %s: %v", name, err)
		}
		mountpoint := mount.NewMountPoint(dev.Path, target, dev.SuperBlock.Type(), unix.MS_NOATIME, "")
		mountpoints.Set(name, mountpoint)
	}

	return mountpoints, nil
}

// MountPointsFromLabels returns the mountpoints required to boot the system.
// Since this function is called exclusively during boot time, this is when
// we want to grow the data filesystem.
func MountPointsFromLabels() (mountpoints *mount.Points, err error) {
	mountpoints = mount.NewMountPoints()
	for _, name := range []string{constants.EphemeralPartitionLabel, constants.BootPartitionLabel} {
		opts := []mount.Option{}
		var target string
		switch name {
		case constants.EphemeralPartitionLabel:
			target = constants.EphemeralMountPoint
			opts = append(opts, mount.WithResize(true))
		case constants.BootPartitionLabel:
			target = constants.BootMountPoint
		}

		var dev *probe.ProbedBlockDevice
		if dev, err = probe.GetDevWithFileSystemLabel(name); err != nil {
			// A bootloader is not always required.
			if name == constants.BootPartitionLabel {
				log.Println("WARNING: no ESP partition was found")
				continue
			}
			return nil, errors.Errorf("find device with label %s: %v", name, err)
		}

		mountpoint := mount.NewMountPoint(dev.Path, target, dev.SuperBlock.Type(), unix.MS_NOATIME, "", opts...)
		mountpoints.Set(name, mountpoint)
	}
	return mountpoints, nil
}
