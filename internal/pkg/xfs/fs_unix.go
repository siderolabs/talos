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

// UnixFS represents a filesystem wrapper for Unix-like systems.
type UnixFS struct {
	Shadow string

	mntfd int
}

// Interface guard.
var _ interface {
	FS
} = (*UnixFS)(nil)

// NewUnix creates a new FS instance with the provided options.
func NewUnix(creator Creator) (*UnixFS, error) {
	unixfs := &UnixFS{}

	mntfd, err := creator.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to create unixfs filesystem: %w", err)
	}

	unixfs.mntfd = mntfd

	return unixfs, nil
}

// Mkdir creates a new directory in the unixfs filesystem with the specified name and permissions.
func (unixfs *UnixFS) Mkdir(name string, perm os.FileMode) error {
	return unix.Mkdirat(unixfs.mntfd, name, uint32(perm))
}

// MountPoint returns the shadow directory of the mounted unixfs filesystem.
func (unixfs *UnixFS) MountPoint() string {
	return unixfs.Shadow
}

// FileDescriptor returns the file descriptor of the mounted unixfs filesystem.
func (unixfs *UnixFS) FileDescriptor() int {
	return unixfs.mntfd
}

// Open opens a file in the unixfs filesystem with the specified name in read-only mode.
func (unixfs *UnixFS) Open(name string) (fs.File, error) {
	return unixfs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file in the unixfs filesystem with the specified name, flags, and permissions.
func (unixfs *UnixFS) OpenFile(name string, flags int, perm os.FileMode) (File, error) {
	fd, err := unix.Openat(unixfs.mntfd, name, flags, uint32(perm))
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), name), nil
}

// Remove removes a file or directory from the unixfs filesystem.
func (unixfs *UnixFS) Remove(name string) error {
	flags := 0

	info, err := unixfs.stat(name)
	if err != nil {
		return err
	}

	if info.IsDir() {
		flags = unix.AT_REMOVEDIR
	}

	return unix.Unlinkat(unixfs.mntfd, name, flags)
}

func (unixfs *UnixFS) stat(name string) (os.FileInfo, error) {
	f, err := unixfs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	return f.Stat()
}
