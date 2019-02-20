/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package install

import (
	"log"

	"github.com/autonomy/talos/internal/pkg/blockdevice/probe"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/autonomy/talos/internal/pkg/mount"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Mount discovers the appropriate partitions by label and mounts them up
// to the appropriate mountpoint.
// TODO: See if we can consolidate this with rootfs/mount
func Mount() (err error) {
	log.Println("Discovering mountpoints")

	var mp *mount.Points
	if mp, err = mountpoints(); err != nil {
		return errors.Errorf("error initializing block devices: %v", err)
	}

	log.Println("Attempting to mount filesystems")
	iter := mp.Iter()
	for iter.Next() {
		log.Println("- ", iter.Value())
		if err = mount.WithRetry(iter.Value(), mount.WithPrefix(constants.NewRoot)); err != nil {
			return errors.Errorf("error mounting partitions: %v", err)
		}
	}
	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}

// nolint: dupl
func mountpoints() (mountpoints *mount.Points, err error) {
	mountpoints = mount.NewMountPoints()
	for _, name := range []string{constants.RootPartitionLabel, constants.DataPartitionLabel, constants.BootPartitionLabel} {
		var target string
		switch name {
		case constants.RootPartitionLabel:
			target = constants.RootMountPoint
		case constants.DataPartitionLabel:
			target = constants.DataMountPoint
		case constants.BootPartitionLabel:
			target = constants.BootMountPoint
		}

		var dev *probe.ProbedBlockDevice
		if dev, err = probe.GetDevWithFileSystemLabel(name); err != nil {
			return nil, errors.Errorf("failed to find device with label %s: %v", name, err)
		}

		mountpoint := mount.NewMountPoint(dev.Path, target, dev.SuperBlock.Type(), unix.MS_NOATIME, "")

		mountpoints.Set(name, mountpoint)
	}

	return mountpoints, nil
}
