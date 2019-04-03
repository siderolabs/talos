/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package mount

import (
	"log"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/filesystem/xfs"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	gptpartition "github.com/talos-systems/talos/internal/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/util"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/cgroups"
	"golang.org/x/sys/unix"
)

// Initializer represents the early boot initialization control.
type Initializer struct {
	prefix string

	owned   *mount.Points
	special *mount.Points
}

// NewInitializer initializes and returns an Initializer struct.
func NewInitializer(prefix string) (initializer *Initializer, err error) {
	special := mount.NewMountPoints()
	special.Set("dev", mount.NewMountPoint("devtmpfs", "/dev", "devtmpfs", unix.MS_NOSUID, "mode=0755"))
	special.Set("proc", mount.NewMountPoint("proc", "/proc", "proc", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""))
	special.Set("sys", mount.NewMountPoint("sysfs", "/sys", "sysfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""))
	special.Set("run", mount.NewMountPoint("tmpfs", "/run", "tmpfs", 0, ""))
	special.Set("tmp", mount.NewMountPoint("tmpfs", "/tmp", "tmpfs", 0, ""))

	initializer = &Initializer{
		prefix:  prefix,
		special: special,
	}

	return initializer, nil
}

// Owned returns the OS owned block devices.
func (i *Initializer) Owned() *mount.Points {
	return i.owned
}

// Special returns the special devices.
func (i *Initializer) Special() *mount.Points {
	return i.special
}

// InitSpecial initializes and mounts the special devices in the early boot
// stage.
func (i *Initializer) InitSpecial() (err error) {
	iter := i.special.Iter()
	for iter.Next() {
		if err = mount.WithRetry(iter.Value()); err != nil {
			return errors.Errorf("error initializing special device at %s: %v", iter.Value().Target(), err)
		}
	}
	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}

// MoveSpecial moves the special device mount points to the new root.
func (i *Initializer) MoveSpecial() (err error) {
	iter := i.special.Iter()
	for iter.Next() {
		mountpoint := mount.NewMountPoint(iter.Value().Target(), iter.Value().Target(), "", unix.MS_MOVE, "")
		if err := mount.WithRetry(mountpoint, mount.WithPrefix(i.prefix)); err != nil {
			return errors.Errorf("error moving mount point %s: %v", iter.Value().Target(), err)
		}
	}
	if iter.Err() != nil {
		return iter.Err()
	}

	if err := mount.WithRetry(mount.NewMountPoint("tmpfs", "/dev/shm", "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME, ""), mount.WithPrefix(i.prefix)); err != nil {
		return errors.Errorf("error mounting mount point %s: %v", "/dev/shm", err)
	}

	if err := mount.WithRetry(mount.NewMountPoint("devpts", "/dev/pts", "devpts", unix.MS_NOSUID|unix.MS_NOEXEC, "ptmxmode=000,mode=620,gid=5"), mount.WithPrefix(i.prefix)); err != nil {
		return errors.Errorf("error mounting mount point %s: %v", "/dev/pts", err)
	}

	return nil
}

// InitOwned initializes and mounts the OS owned block devices in the early boot
// stage.
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
		if err = xfs.GrowFS(path.Join(i.prefix, mountpoint.Target())); err != nil {
			return errors.Errorf("error growing data partition file system: %v", err)
		}
	}

	return nil
}

// MountOwned mounts the OS owned block devices.
func (i *Initializer) MountOwned() (err error) {
	iter := i.owned.Iter()
	for iter.Next() {
		if iter.Key() == constants.RootPartitionLabel {
			if err = mount.WithRetry(iter.Value(), mount.WithPrefix(i.prefix), mount.WithReadOnly(true), mount.WithShared(true)); err != nil {
				return errors.Errorf("error mounting partitions: %v", err)
			}
		} else {
			if err = mount.WithRetry(iter.Value(), mount.WithPrefix(i.prefix)); err != nil {
				return errors.Errorf("error mounting partitions: %v", err)
			}
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

// Switch moves the root to a specified directory. See
// https://github.com/karelzak/util-linux/blob/master/sys-utils/switch_root.c.
// nolint: gocyclo
func (i *Initializer) Switch() (err error) {
	// Unmount the ROOT and DATA block devices.
	if err = i.UnmountOwned(); err != nil {
		return err
	}

	// Mount the ROOT and DATA block devices at the new root.
	if err = i.MountOwned(); err != nil {
		return errors.Wrap(err, "error mounting block device")
	}

	// Move the special mount points to the new root.
	if err = i.MoveSpecial(); err != nil {
		return errors.Wrap(err, "error moving special devices")
	}

	// Mount the cgroups to the new root.
	if err = cgroups.Mount(i.prefix); err != nil {
		return errors.Wrap(err, "error mounting cgroups")
	}

	if err = unix.Chdir(i.prefix); err != nil {
		return errors.Wrapf(err, "error changing working directory to %s", i.prefix)
	}

	var old *os.File
	if old, err = os.Open("/"); err != nil {
		return errors.Wrap(err, "error opening /")
	}
	// nolint: errcheck
	defer old.Close()

	if err = unix.Mount(i.prefix, "/", "", unix.MS_MOVE, ""); err != nil {
		return errors.Wrap(err, "error moving /")
	}

	if err = unix.Chroot("."); err != nil {
		return errors.Wrap(err, "error chroot")
	}

	if err = recursiveDelete(int(old.Fd())); err != nil {
		return errors.Wrap(err, "error deleting initramfs")
	}

	if err = unix.Exec("/proc/self/exe", []string{"exe", "--switch-root"}, []string{}); err != nil {
		return errors.Wrap(err, "error executing /proc/self/exe")
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

func recursiveDelete(fd int) error {
	parentDev, err := getDev(fd)
	if err != nil {
		return err
	}

	dir := os.NewFile(uintptr(fd), "__ignored__")
	// nolint: errcheck
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, name := range names {
		if err := recusiveDeleteInner(fd, parentDev, name); err != nil {
			return err
		}
	}
	return nil
}

func recusiveDeleteInner(parentFd int, parentDev uint64, childName string) error {
	childFd, err := unix.Openat(parentFd, childName, unix.O_DIRECTORY|unix.O_NOFOLLOW, unix.O_RDWR)
	if err != nil {
		if err := unix.Unlinkat(parentFd, childName, 0); err != nil {
			return err
		}
	} else {
		// nolint: errcheck
		defer unix.Close(childFd)

		if childFdDev, err := getDev(childFd); err != nil {
			return err
		} else if childFdDev != parentDev {
			return nil
		}

		if err := recursiveDelete(childFd); err != nil {
			return err
		}
		if err := unix.Unlinkat(parentFd, childName, unix.AT_REMOVEDIR); err != nil {
			return err
		}
	}
	return nil
}

func getDev(fd int) (dev uint64, err error) {
	var stat unix.Stat_t

	if err := unix.Fstat(fd, &stat); err != nil {
		return 0, err
	}

	return stat.Dev, nil
}
