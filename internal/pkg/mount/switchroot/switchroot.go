// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package switchroot

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/mount"
)

// Switch moves the rootfs to a specified directory. See
// https://github.com/karelzak/util-linux/blob/master/sys-utils/switch_root.c.
func Switch(prefix string, mountpoints *mount.Points) (err error) {
	log.Println("moving mounts to the new rootfs")

	if err = mount.Move(mountpoints, prefix); err != nil {
		return err
	}

	log.Printf("changing working directory into %s", prefix)

	if err = unix.Chdir(prefix); err != nil {
		return fmt.Errorf("error changing working directory to %s: %w", prefix, err)
	}

	var old *os.File

	if old, err = os.Open("/"); err != nil {
		return fmt.Errorf("error opening /: %w", err)
	}

	//nolint:errcheck
	defer old.Close()

	log.Printf("moving %s to /", prefix)

	if err = unix.Mount(prefix, "/", "", unix.MS_MOVE, ""); err != nil {
		return fmt.Errorf("error moving /: %w", err)
	}

	log.Println("changing root directory")

	if err = unix.Chroot("."); err != nil {
		return fmt.Errorf("error chroot: %w", err)
	}

	log.Println("cleaning up initramfs")

	if err = recursiveDelete(int(old.Fd())); err != nil {
		return fmt.Errorf("error deleting initramfs: %w", err)
	}

	// Note that /sbin/init is machined. We call it init since this is the
	// convention.
	log.Println("executing /sbin/init")

	if err = unix.Exec("/sbin/init", []string{"/sbin/init"}, []string{}); err != nil {
		return fmt.Errorf("error executing /sbin/init: %w", err)
	}

	return nil
}

func recursiveDelete(fd int) error {
	parentDev, err := getDev(fd)
	if err != nil {
		return err
	}

	dir := os.NewFile(uintptr(fd), "__ignored__")
	//nolint:errcheck
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
		//nolint:errcheck
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
