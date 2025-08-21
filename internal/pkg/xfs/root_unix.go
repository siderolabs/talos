// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package xfs

import (
	"fmt"
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

// UnixRoot represents a filesystem wrapper for Unix-like systems.
type UnixRoot struct {
	Shadow string
	FS     FS

	mntfd int
}

// Interface guard.
var _ interface {
	Root
} = (*UnixRoot)(nil)

// OpenFS opens the underlying filesystem.
func (root *UnixRoot) OpenFS() error {
	var err error

	if root.mntfd != 0 {
		return nil
	}

	root.mntfd, err = root.FS.Open()
	if err != nil {
		return fmt.Errorf("failed to create root filesystem: %w", err)
	}

	return nil
}

// Close closes the underlying filesystem.
func (root *UnixRoot) Close() error {
	if root.mntfd == 0 {
		return nil
	}

	root.mntfd = 0

	return root.FS.Close()
}

// RepairFS repairs the underlying filesystem if necessary.
func (root *UnixRoot) RepairFS() error {
	return root.FS.Repair()
}

// Fd returns the file descriptor of the mounted root filesystem.
// It returns an error if the filesystem is not open or has been closed.
// Usage of the returned file descriptior is no longer thread safe.
func (root *UnixRoot) Fd() (int, error) {
	if root.mntfd == 0 {
		return 0, os.ErrClosed
	}

	return root.mntfd, nil
}

// Mkdir creates a new directory in the root filesystem with the specified name and permissions.
func (root *UnixRoot) Mkdir(name string, perm os.FileMode) error {
	return unix.Mkdirat(root.mntfd, name, uint32(perm))
}

// Open opens a file in the root filesystem with the specified name in read-only mode.
func (root *UnixRoot) Open(name string) (fs.File, error) {
	return root.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file in the root filesystem with the specified name, flags, and permissions.
func (root *UnixRoot) OpenFile(name string, flags int, perm os.FileMode) (File, error) {
	fd, err := unix.Openat(root.mntfd, name, flags, uint32(perm))
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), name), nil
}

// Remove removes a file or directory from the root filesystem.
func (root *UnixRoot) Remove(name string) error {
	flags := 0

	info, err := root.stat(name)
	if err != nil {
		return err
	}

	if info.IsDir() {
		flags = unix.AT_REMOVEDIR
	}

	return unix.Unlinkat(root.mntfd, name, flags)
}

func (root *UnixRoot) stat(name string) (os.FileInfo, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	return f.Stat()
}

// Source returns the source of the underlying filesystem.
func (root *UnixRoot) Source() string {
	return root.FS.Source()
}

// FSType returns the type of the underlying filesystem.
func (root *UnixRoot) FSType() string {
	return root.FS.FSType()
}
