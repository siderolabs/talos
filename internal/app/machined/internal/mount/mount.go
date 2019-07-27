/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package mount

import (
	"fmt"
	"log"
	"path"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/filesystem/xfs"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	gptpartition "github.com/talos-systems/talos/internal/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/util"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/userdata"
	"golang.org/x/sys/unix"
)

// Initializer represents the early boot initialization control.
type Initializer struct {
	prefix string

	owned *mount.Points
}

// NewInitializer initializes and returns an Initializer struct.
func NewInitializer(prefix string) (initializer *Initializer, err error) {
	initializer = &Initializer{
		prefix: prefix,
	}

	return initializer, nil
}

// Owned returns the OS owned block devices.
func (i *Initializer) Owned() *mount.Points {
	return i.owned
}

// InitOwned initializes and mounts the OS owned block devices in the early boot
// tasks.
func (i *Initializer) InitOwned() (err error) {
	var owned *mount.Points
	if owned, err = mountpoints(); err != nil {
		return errors.Errorf("error initializing owned block devices: %v", err)
	}
	i.owned = owned
	if mountpoint, ok := i.owned.Get(constants.DataPartitionLabel); ok {
		if err = repair(mountpoint); err != nil {
			return errors.Errorf("error fixing data partition: %v", err)
		}
	}

	iter := i.owned.Iter()
	for iter.Next() {
		if err = mount.WithRetry(iter.Value(), mount.WithPrefix(i.prefix)); err != nil {
			return errors.Errorf("error mounting partitions: %v", err)
		}
	}
	if iter.Err() != nil {
		return iter.Err()
	}

	if mountpoint, ok := i.owned.Get(constants.DataPartitionLabel); ok {
		// NB: The XFS partition MUST be mounted, or this will fail.
		log.Printf("growing the %s partition", constants.DataPartitionLabel)
		if err = xfs.GrowFS(path.Join(i.prefix, mountpoint.Target())); err != nil {
			return errors.Errorf("error growing data partition file system: %v", err)
		}
	}

	return nil
}

// ExtraDevices mounts the extra devices.
func ExtraDevices(data *userdata.UserData) (err error) {
	if data.Install == nil || data.Install.ExtraDevices == nil {
		return nil
	}
	for _, extra := range data.Install.ExtraDevices {
		for i, part := range extra.Partitions {
			devname := fmt.Sprintf("%s%d", extra.Device, i+1)
			mountpoint := mount.NewMountPoint(devname, part.MountPoint, "xfs", unix.MS_NOATIME, "")
			if err = mount.WithRetry(mountpoint); err != nil {
				return errors.Errorf("failed to mount %s at %s: %v", devname, part.MountPoint, err)
			}
		}
	}

	return nil
}

// MountOwned mounts the OS owned block devices.
func (i *Initializer) MountOwned() (err error) {
	iter := i.owned.Iter()
	for iter.Next() {
		if err = mount.WithRetry(iter.Value(), mount.WithPrefix(i.prefix)); err != nil {
			return errors.Errorf("error mounting partitions: %v", err)
		}
	}
	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}

// UnmountOwned unmounts the OS owned block devices.
func (i *Initializer) UnmountOwned() (err error) {
	iter := i.owned.IterRev()
	for iter.Next() {
		if err = mount.UnWithRetry(iter.Value(), mount.WithPrefix(i.prefix)); err != nil {
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
