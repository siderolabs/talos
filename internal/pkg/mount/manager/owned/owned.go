// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package owned

import (
	"fmt"
	"log"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/constants"
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
				log.Println("WARNING: no boot partition was found")
				continue
			}

			return nil, fmt.Errorf("probe device for filesystem %s: %w", name, err)
		}

		// nolint: errcheck
		defer dev.Close()

		mountpoint := mount.NewMountPoint(dev.Path, target, dev.SuperBlock.Type(), unix.MS_NOATIME, "")
		mountpoints.Set(name, mountpoint)
	}

	return mountpoints, nil
}

// MountPointForLabel returns a mount point for the specified device and label.
func MountPointForLabel(label string, opts ...mount.Option) (mountpoint *mount.Point, err error) {
	var target string

	switch label {
	case constants.EphemeralPartitionLabel:
		target = constants.EphemeralMountPoint

		opts = append(opts, mount.WithResize(true))
	case constants.BootPartitionLabel:
		target = constants.BootMountPoint
	default:
		return nil, fmt.Errorf("unknown label: %q", label)
	}

	var dev *probe.ProbedBlockDevice

	if dev, err = probe.GetDevWithFileSystemLabel(label); err != nil {
		// A boot partitition is not required.
		if label == constants.BootPartitionLabel {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to find device with label %s: %w", label, err)
	}

	// nolint: errcheck
	defer dev.Close()

	mountpoint = mount.NewMountPoint(dev.Path, target, dev.SuperBlock.Type(), unix.MS_NOATIME, "", opts...)

	return mountpoint, nil
}
