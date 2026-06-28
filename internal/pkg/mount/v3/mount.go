// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mount handles filesystem mount operations.
package mount

import (
	"fmt"
	"io"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/xfs"
)

// bindHardenAttr is the baseline attribute set every read-only bind mount
// inherits: read-only, no setuid escalation, no device nodes (per
// siderolabs/talos#11946 — device nodes belong only in /dev and /dev/pts).
const bindHardenAttr = unix.MOUNT_ATTR_RDONLY | unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NOEXEC | unix.MOUNT_ATTR_NODEV

// ClonedInfo is a detached, pathless mount returned by OpenTreeClone. It is reachable only through its fd and must be closed to release the clone.
type ClonedInfo interface {
	Fd() int
	io.Closer
}

type cloneInfo struct {
	fd int
}

func (cm *cloneInfo) Fd() int {
	return cm.fd
}

// Close releases the cloned mount.
func (cm *cloneInfo) Close() error {
	if cm.fd == -1 {
		return fmt.Errorf("cloned mount already closed")
	}

	if err := unix.Close(cm.fd); err != nil {
		return fmt.Errorf("failed to close cloned mount: %w", err)
	}

	cm.fd = -1

	return nil
}

// OpenTreeClone clones the mount at path into a detached, pathless mount via
// open_tree(OPEN_TREE_CLONE). The returned fd is reachable only through that fd (e.g. usable as an
// overlay lower layer); the returned io.Closer releases it and should be deferred by the caller.
func OpenTreeClone(path string) (ClonedInfo, error) {
	fd, err := unix.OpenTree(unix.AT_FDCWD, path, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("failed to open_tree %q: %w", path, err)
	}

	return &cloneInfo{fd}, nil
}

// BindReadonly creates a common way to create a readonly bind mounted destination.
func BindReadonly(src, dst string) error {
	sourceFD, err := unix.OpenTree(unix.AT_FDCWD, src, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC)
	if err != nil {
		return fmt.Errorf("failed to opentree source %s: %w", src, err)
	}

	defer unix.Close(sourceFD) //nolint:errcheck

	if err := unix.MountSetattr(sourceFD, "", unix.AT_EMPTY_PATH, &unix.MountAttr{
		Attr_set: bindHardenAttr,
	}); err != nil {
		return fmt.Errorf("failed to set mount attribute: %w", err)
	}

	if err := unix.MoveMount(sourceFD, "", unix.AT_FDCWD, dst, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("failed to move mount from %s to %s: %w", src, dst, err)
	}

	return nil
}

// BindReadonlyFd creates a common way to create a readonly bind mounted destination.
func BindReadonlyFd(dfd int, dst string) error {
	sourceFD, err := unix.OpenTree(dfd, "", unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC|unix.AT_EMPTY_PATH)
	if err != nil {
		return fmt.Errorf("failed to opentree: %w", err)
	}

	defer unix.Close(sourceFD) //nolint:errcheck

	if err := unix.MountSetattr(sourceFD, "", unix.AT_EMPTY_PATH, &unix.MountAttr{
		Attr_set: bindHardenAttr,
	}); err != nil {
		return fmt.Errorf("failed to set mount attribute: %w", err)
	}

	if err := unix.MoveMount(sourceFD, "", unix.AT_FDCWD, dst, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("failed to move mount to %s: %w", dst, err)
	}

	return nil
}

// BindRootPath binds a path inside root to dst.
func BindRootPath(root xfs.Root, name, dst string, attrs int) error {
	rootFD, err := root.Fd()
	if err != nil {
		return fmt.Errorf("failed to get root fd: %w", err)
	}

	sourceFD, err := unix.OpenTree(rootFD, name, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC)
	if err != nil {
		return fmt.Errorf("failed to opentree %q: %w", name, err)
	}

	defer unix.Close(sourceFD) //nolint:errcheck

	if attrs != 0 {
		if err := unix.MountSetattr(sourceFD, "", unix.AT_EMPTY_PATH, &unix.MountAttr{
			Attr_set: uint64(attrs),
		}); err != nil {
			return fmt.Errorf("failed to set mount attribute: %w", err)
		}
	}

	if err := unix.MoveMount(sourceFD, "", unix.AT_FDCWD, dst, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("failed to move mount to %s: %w", dst, err)
	}

	return nil
}
