// +build linux

package switchroot

import (
	"os"
	"syscall"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/mount"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/mount/cgroups"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func recursiveDelete(fd int) error {
	parentDev, err := getDev(fd)
	if err != nil {
		return err
	}

	// The file descriptor is already open, but allocating a os.File here makes
	// reading the files in the dir so much nicer.
	dir := os.NewFile(uintptr(fd), "__ignored__")
	// nolint: errcheck
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, name := range names {
		// Loop here, but handle loop in separate function to make defer work as
		// expected.
		if err := recusiveDeleteInner(fd, parentDev, name); err != nil {
			return err
		}
	}
	return nil
}

func recusiveDeleteInner(parentFd int, parentDev uint64, childName string) error {
	// O_DIRECTORY and O_NOFOLLOW make this open fail for all files and all
	// symlinks (even when pointing to a dir). We need to filter out symlinks
	// because getDev later follows them.
	childFd, err := unix.Openat(parentFd, childName, unix.O_DIRECTORY|unix.O_NOFOLLOW, unix.O_RDWR)
	if err != nil {
		// childName points to either a file or a symlink, delete in any case.
		if err := unix.Unlinkat(parentFd, childName, 0); err != nil {
			return err
		}
	} else {
		// Open succeeded, which means childName points to a real directory.
		// nolint: errcheck
		defer unix.Close(childFd)

		// Don't descent into other file systems.
		if childFdDev, err := getDev(childFd); err != nil {
			return err
		} else if childFdDev != parentDev {
			// This means continue in recursiveDelete.
			return nil
		}

		if err := recursiveDelete(childFd); err != nil {
			return err
		}
		// Back from recursion, the directory is now empty, delete.
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

// Switch performs a switch_root equivalent. See
// https://github.com/karelzak/util-linux/blob/master/sys-utils/switch_root.c
func Switch(s string) error {
	// Mount the ROOT and DATA block devices at the new root.
	if err := mount.Mount(s); err != nil {
		return errors.Wrap(err, "error mounting block device")
	}
	// Move the special mount points to the new root.
	if err := mount.Move(s); err != nil {
		return errors.Wrap(err, "error moving special devices")
	}
	// Mount the cgroups file systems to the new root.
	if err := cgroups.Mount(s); err != nil {
		return errors.Wrap(err, "error mounting cgroups")
	}
	if err := unix.Chdir(s); err != nil {
		return errors.Wrapf(err, "error changing working directory to %s", s)
	}
	oldRoot, err := os.Open("/")
	if err != nil {
		return errors.Wrap(err, "error opening /")
	}
	// nolint: errcheck
	defer oldRoot.Close()
	if err := mount.Finalize(s); err != nil {
		return errors.Wrap(err, "error moving /")
	}
	if err := unix.Chroot("."); err != nil {
		return errors.Wrap(err, "error chroot")
	}
	if err := recursiveDelete(int(oldRoot.Fd())); err != nil {
		return errors.Wrap(err, "error deleting initramfs")
	}
	if err := syscall.Exec("/proc/self/exe", []string{"exe", "--switch-root"}, []string{}); err != nil {
		return errors.Wrap(err, "error executing /proc/self/exe")
	}

	return nil
}
