// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/xfs"
)

// Point represents a mount point in the filesystem.
type Point struct {
	root         xfs.Root
	keepOpen     bool
	detached     bool
	fstype       string
	source       string
	target       string
	selinuxLabel string
}

// NewPoint creates a new mount point.
func NewPoint(source string, srcfd int, target string, targetfd int, fstype string) *Point {
	return &Point{
		source: source,
		target: target,
		fstype: fstype,
	}
}

// Options represents options for mounting a mount point.
type Options struct {
	SkipIfMounted   bool
	MountAttributes int
	Shared          bool
	Printer         func(string, ...any)
}

// Mount the mount point.
//
//nolint:gocyclo
func (p *Point) Mount(opts Options) error {
	defer p.Release(false) //nolint:errcheck

	if p.detached {
		return nil
	}

	if opts.SkipIfMounted {
		isMounted, err := p.IsMounted()
		if err != nil {
			return err
		}

		if isMounted {
			return nil
		}
	}

	return p.retry(func() error {
		if err := p.moveMount(p.target); err != nil {
			return fmt.Errorf("error mounting %q to %q: %w", p.Source(), p.target, err)
		}

		if err := p.setattr(&unix.MountAttr{
			Attr_set: uint64(opts.MountAttributes),
		}, 0); err != nil {
			return fmt.Errorf("error setting mountattributes on %s: %w", p.target, err)
		}

		if opts.Shared {
			if err := p.Share(); err != nil {
				return fmt.Errorf("error making %q shared: %w", p.target, err)
			}
		}

		if err := selinux.SetLabel(p.target, p.selinuxLabel); err != nil && !errors.Is(err, unix.ENOTSUP) {
			return fmt.Errorf("error setting selinux label on %q: %w", p.target, err)
		}

		return nil
	}, false)
}

// Share makes the mount point shared.
func (p *Point) Share() error {
	if p.detached {
		return syscall.EINVAL
	}

	return p.setattr(&unix.MountAttr{
		Propagation: unix.MS_SHARED,
	}, unix.AT_RECURSIVE)
}

// UnmountOptions represents options for unmounting a mount point.
type UnmountOptions struct {
	Printer func(string, ...any)
}

// Release closes the file descriptor of the underlying mount point.
func (p *Point) Release(force bool) error {
	if p.keepOpen && !force {
		return nil
	}

	return p.root.Close()
}

// Unmount unmounts the mount point, retrying on certain errors.
func (p *Point) Unmount(opts UnmountOptions) error {
	if err := p.Release(true); err != nil {
		return err
	}

	if p.detached {
		return nil
	}

	return p.retry(func() error {
		return SafeUnmount(context.Background(), opts.Printer, p.target)
	}, true)
}

// IsMounted checks if the mount point is mounted by checking the mount on the target.
func (p *Point) IsMounted() (bool, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false, err
	}

	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) < 2 {
			continue
		}

		mountpoint := fields[1]

		if mountpoint == p.target {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// Move the mount point to a new target.
func (p *Point) Move(newTarget string) error {
	return p.moveMount(newTarget)
}

//nolint:gocyclo
func (p *Point) retry(f func() error, isUnmount bool) error {
	return retry.Constant(5*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(func() error {
		if err := f(); err != nil {
			switch err {
			case unix.EBUSY:
				return retry.ExpectedError(err)
			case unix.ENOENT, unix.ENXIO:
				// if udevd triggers BLKRRPART ioctl, partition device entry might disappear temporarily
				return retry.ExpectedError(err)
			case unix.EUCLEAN, unix.EIO:
				if !isUnmount {
					if errRepair := p.root.RepairFS(); errRepair != nil {
						return fmt.Errorf("error repairing: %w", errRepair)
					}
				}

				return retry.ExpectedError(err)
			case unix.EINVAL:
				isMounted, checkErr := p.IsMounted()
				if checkErr != nil {
					return retry.ExpectedError(checkErr)
				}

				if !isMounted && !isUnmount {
					if errRepair := p.root.RepairFS(); errRepair != nil {
						return fmt.Errorf("error repairing: %w", errRepair)
					}

					return retry.ExpectedError(err)
				}

				if !isMounted && isUnmount { // if partition is already unmounted, ignore EINVAL
					return nil
				}

				return err
			default:
				return err
			}
		}

		return nil
	})
}

func (p *Point) moveMount(target string) error {
	fd, err := p.root.Fd()
	if err != nil {
		if p.Source() != "" {
			if err := unix.MoveMount(unix.AT_FDCWD, p.Source(), unix.AT_FDCWD, target, 0); err != nil {
				return fmt.Errorf("error moving mount from %q to %q: %w", p.Source(), target, err)
			}

			return nil
		}

		return fmt.Errorf("error getting root fd: %w", err)
	}

	if err := unix.MoveMount(fd, "", unix.AT_FDCWD, target, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("error moving detached mount fd=%d to %q: %w", fd, target, err)
	}

	return nil
}

// Target returns the target of the mount point.
func (p *Point) Target() string {
	return p.target
}

// Source returns the source of the mount point.
func (p *Point) Source() string {
	if p.source == "" {
		return p.root.Source()
	}

	return p.source
}

// FSType returns the filesystem type of the mount point.
func (p *Point) FSType() string {
	if p.fstype == "" {
		return p.root.FSType()
	}

	return p.fstype
}

// Root returns the underlying xfs.Root of the mount point.
func (p *Point) Root() xfs.Root {
	return p.root
}

// RemountReadOnly remounts the mount point as read-only.
func (p *Point) RemountReadOnly() error {
	if p.detached {
		return nil
	}

	return p.setattr(&unix.MountAttr{
		Attr_set: unix.MOUNT_ATTR_RDONLY,
	}, 0)
}

// RemountReadWrite remounts the mount point as read-write.
func (p *Point) RemountReadWrite() error {
	if p.detached {
		return nil
	}

	return p.setattr(&unix.MountAttr{
		Attr_clr: unix.MOUNT_ATTR_RDONLY,
	}, 0)
}

// SetDisableAccessTime sets or clears the noatime mount attribute.
func (p *Point) SetDisableAccessTime(disable bool) error {
	if p.detached {
		return nil
	}

	if disable {
		return p.setattr(&unix.MountAttr{
			Attr_set: unix.MOUNT_ATTR_NOATIME,
		}, 0)
	}

	return p.setattr(&unix.MountAttr{
		Attr_clr: unix.MOUNT_ATTR_NOATIME,
	}, 0)
}

// SetSecure sets or clears the nosuid and nodev mount attributes.
func (p *Point) SetSecure(secure bool) error {
	if p.detached {
		return nil
	}

	if secure {
		return p.setattr(&unix.MountAttr{
			Attr_set: unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NODEV,
		}, 0)
	}

	return p.setattr(&unix.MountAttr{
		Attr_clr: unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NODEV,
	}, 0)
}

func (p *Point) setattr(attr *unix.MountAttr, flags int) error {
	if (attr.Attr_set&unix.MOUNT_ATTR_NOATIME) != 0 || (attr.Attr_clr&unix.MOUNT_ATTR_NOATIME) != 0 {
		attr.Attr_clr |= unix.MOUNT_ATTR__ATIME
	}

	fd, err := p.root.Fd()
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return err
	}

	if p.target != "" {
		fd = unix.AT_FDCWD
	} else {
		flags |= unix.AT_EMPTY_PATH
	}

	if err := unix.MountSetattr(fd, p.target, uint(flags), attr); err != nil {
		return fmt.Errorf("setattr failed for fd=%d target=%q flags=%d attr=%#+v: %w", fd, p.target, flags, *attr, err)
	}

	return nil
}
