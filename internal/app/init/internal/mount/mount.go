/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package mount

import (
	"os"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"golang.org/x/sys/unix"
	"gopkg.in/freddierice/go-losetup.v1"
)

// Initializer represents the early boot initialization control.
type Initializer struct {
	prefix string

	special *mount.Points
}

// NewInitializer initializes and returns an Initializer struct.
func NewInitializer(prefix string) (initializer *Initializer, err error) {
	special := mount.NewMountPoints()
	special.Set("dev", mount.NewMountPoint("devtmpfs", "/dev", "devtmpfs", unix.MS_NOSUID, "mode=0755"))
	special.Set("proc", mount.NewMountPoint("proc", "/proc", "proc", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""))
	special.Set("sys", mount.NewMountPoint("sysfs", "/sys", "sysfs", 0, ""))
	special.Set("run", mount.NewMountPoint("tmpfs", "/run", "tmpfs", 0, ""))
	special.Set("tmp", mount.NewMountPoint("tmpfs", "/tmp", "tmpfs", 0, ""))

	initializer = &Initializer{
		prefix:  prefix,
		special: special,
	}

	return initializer, nil
}

// Special returns the special devices.
func (i *Initializer) Special() *mount.Points {
	return i.special
}

// InitSpecial initializes and mounts the special devices in the early boot
// tasks.
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

// Rootfs initializes and mounts the OS owned block devices in the early boot
// tasks.
func (i *Initializer) Rootfs() (err error) {
	var dev losetup.Device
	dev, err = losetup.Attach("/"+constants.RootfsAsset, 0, true)
	if err != nil {
		return err
	}

	m := mount.NewMountPoint(dev.Path(), "/", "squashfs", unix.MS_RDONLY, "")
	if err = mount.WithRetry(m, mount.WithPrefix(i.prefix), mount.WithReadOnly(true), mount.WithShared(true)); err != nil {
		return errors.Wrap(err, "failed to mount squashfs")
	}

	return nil
}

// MoveSpecial moves the special device mount points to the new root.
func (i *Initializer) MoveSpecial() (err error) {
	iter := i.special.Iter()
	for iter.Next() {
		mountpoint := mount.NewMountPoint(iter.Value().Target(), iter.Value().Target(), "", unix.MS_MOVE, "")
		if err = mount.WithRetry(mountpoint, mount.WithPrefix(i.prefix)); err != nil {
			return errors.Errorf("error moving mount point %s: %v", iter.Value().Target(), err)
		}
	}
	if iter.Err() != nil {
		return iter.Err()
	}

	if err = mount.WithRetry(mount.NewMountPoint("tmpfs", "/dev/shm", "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME, ""), mount.WithPrefix(i.prefix)); err != nil {
		return errors.Errorf("error mounting mount point %s: %v", "/dev/shm", err)
	}

	if err = mount.WithRetry(mount.NewMountPoint("devpts", "/dev/pts", "devpts", unix.MS_NOSUID|unix.MS_NOEXEC, "ptmxmode=000,mode=620,gid=5"), mount.WithPrefix(i.prefix)); err != nil {
		return errors.Errorf("error mounting mount point %s: %v", "/dev/pts", err)
	}

	return nil
}

// Switch moves the root to a specified directory. See
// https://github.com/karelzak/util-linux/blob/master/sys-utils/switch_root.c.
// nolint: gocyclo
func (i *Initializer) Switch() (err error) {
	if err = i.MoveSpecial(); err != nil {
		return errors.Wrap(err, "error moving special devices")
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

	// Note that /sbin/init is machined. We call it init since this is the
	// convention.
	if err = unix.Exec("/sbin/init", []string{"/sbin/init"}, []string{}); err != nil {
		return errors.Wrap(err, "error executing /sbin/init")
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
