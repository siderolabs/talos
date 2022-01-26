// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package switchroot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/talos-systems/go-debug"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Paths preserved in the initramfs.
var preservedPaths = map[string]struct{}{
	constants.ExtensionsConfigFile: {},
	constants.FirmwarePath:         {},
}

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

	if _, err = recursiveDelete(int(old.Fd()), "/"); err != nil {
		return fmt.Errorf("error deleting initramfs: %w", err)
	}

	// Note that /sbin/init is machined. We call it init since this is the
	// convention.
	log.Println("executing /sbin/init")

	envv := []string{}

	if debug.RaceEnabled {
		envv = append(envv, "GORACE=halt_on_error=1")

		log.Printf("race detection enabled with halt_on_error=1")
	}

	if err = unix.Exec("/sbin/init", []string{"/sbin/init"}, envv); err != nil {
		return fmt.Errorf("error executing /sbin/init: %w", err)
	}

	return nil
}

func recursiveDelete(fd int, path string) (preserved bool, err error) {
	parentDev, err := getDev(fd)
	if err != nil {
		return false, err
	}

	dir := os.NewFile(uintptr(fd), "__ignored__")
	//nolint:errcheck
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return false, err
	}

	preserved = false

	for _, name := range names {
		p, err := recusiveDeleteInner(fd, parentDev, name, filepath.Join(path, name))
		if err != nil {
			return false, err
		}

		preserved = preserved || p
	}

	return preserved, nil
}

func recusiveDeleteInner(parentFd int, parentDev uint64, childName, path string) (preserved bool, err error) {
	if _, preserved = preservedPaths[path]; preserved {
		return preserved, nil
	}

	childFd, err := unix.Openat(parentFd, childName, unix.O_DIRECTORY|unix.O_NOFOLLOW, unix.O_RDWR)
	if err != nil {
		return false, unix.Unlinkat(parentFd, childName, 0)
	}

	//nolint:errcheck
	defer unix.Close(childFd)

	var childFdDev uint64

	if childFdDev, err = getDev(childFd); err != nil {
		return false, err
	} else if childFdDev != parentDev {
		return false, nil
	}

	preserved, err = recursiveDelete(childFd, path)
	if err != nil {
		return false, err
	}

	if preserved {
		// some child paths got preserved, skip unlinking the parent
		return preserved, nil
	}

	err = unix.Unlinkat(parentFd, childName, unix.AT_REMOVEDIR)

	return false, err
}

func getDev(fd int) (dev uint64, err error) {
	var stat unix.Stat_t

	if err := unix.Fstat(fd, &stat); err != nil {
		return 0, err
	}

	return stat.Dev, nil
}
