/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package owned

import (
	"log"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"golang.org/x/sys/unix"
)

// MountPointsForDevice returns the mountpoints required to boot the system.
func MountPointsForDevice(devpath string) (mountpoints *mount.Points, err error) {
	mountpoints = mount.NewMountPoints()
	for _, name := range []string{constants.DataPartitionLabel, constants.BootPartitionLabel} {
		var target string
		switch name {
		case constants.DataPartitionLabel:
			target = constants.DataMountPoint
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
func MountPointsFromLabels() (mountpoints *mount.Points, err error) {
	mountpoints = mount.NewMountPoints()
	for _, name := range []string{constants.DataPartitionLabel, constants.BootPartitionLabel} {
		var target string
		switch name {
		case constants.DataPartitionLabel:
			target = constants.DataMountPoint
		case constants.BootPartitionLabel:
			target = constants.BootMountPoint
		}

		var dev *probe.ProbedBlockDevice
		if dev, err = probe.GetDevWithFileSystemLabel(name); err != nil {
			if name == constants.BootPartitionLabel {
				// A bootloader is not always required.
				log.Println("WARNING: no ESP partition was found")
				continue
			}
			return nil, errors.Errorf("find device with label %s: %v", name, err)
		}
		mountpoint := mount.NewMountPoint(dev.Path, target, dev.SuperBlock.Type(), unix.MS_NOATIME, "")
		mountpoints.Set(name, mountpoint)
	}
	return mountpoints, nil
}
