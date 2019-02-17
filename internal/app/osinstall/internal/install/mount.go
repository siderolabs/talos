/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package install

import (
	"log"

	"github.com/autonomy/talos/internal/pkg/blockdevice"
	"github.com/autonomy/talos/internal/pkg/blockdevice/probe"
	gptpartition "github.com/autonomy/talos/internal/pkg/blockdevice/table/gpt/partition"
	"github.com/autonomy/talos/internal/pkg/blockdevice/util"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/autonomy/talos/internal/pkg/mount"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func Mount() (err error) {
	log.Println("Discovering mountpoints")
	var mp *mount.Points
	if mp, err = mountpoints(); err != nil {
		return errors.Errorf("error initializing block devices: %v", err)
	}

	/*
		// This seems to be corrupting the filesystem
		// Running xfs_repair finds a bunch of badness

		log.Println("Repairing mountpoints")
		for _, name := range []string{constants.RootPartitionLabel, constants.DataPartitionLabel, constants.BootPartitionLabel} {

			if mountpoint, ok := mp.Get(name); ok {
				if err = repair(mountpoint); err != nil {
					return errors.Errorf("error fixing %s partition: %v", name, err)
				}
			}
		}
	*/

	log.Println("Attempting to mount filesystems")
	iter := mp.Iter()
	for iter.Next() {
		if err = mount.WithRetry(iter.Value(), mount.WithPrefix(constants.NewRoot)); err != nil {
			return errors.Errorf("error mounting partitions: %v", err)
		}
	}
	if iter.Err() != nil {
		return iter.Err()
	}

	/*
		log.Println("Attempting to grow data partition")
		if mountpoint, ok := mp.Get(constants.DataPartitionLabel); ok {
			// NB: The XFS partition MUST be mounted, or this will fail.
			if err = xfs.GrowFS(path.Join(constants.NewRoot, mountpoint.Target())); err != nil {
				return errors.Errorf("error growing data partition file system: %v", err)
			}
		}
	*/

	return nil
}

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

func repair(mountpoint *mount.Point) (err error) {
	var devname string
	if devname, err = util.DevnameFromPartname(mountpoint.Source()); err != nil {
		return err
	}
	bd, err := blockdevice.Open("/dev/" + devname)
	if err != nil {
		return errors.Errorf("error opening block device %q: %v", devname, err)
	}
	// nolint: errcheck
	defer bd.Close()

	pt, err := bd.PartitionTable(true)
	if err != nil {
		return err
	}

	if err := pt.Repair(); err != nil {
		return err
	}

	for _, partition := range pt.Partitions() {
		if partition.(*gptpartition.Partition).Name == constants.DataPartitionLabel {
			if err := pt.Resize(partition); err != nil {
				return err
			}
		}
	}

	if err := pt.Write(); err != nil {
		return err
	}

	// Rereading the partition table requires that all partitions be unmounted
	// or it will fail with EBUSY.
	if err := bd.RereadPartitionTable(); err != nil {
		return err
	}

	return nil
}
