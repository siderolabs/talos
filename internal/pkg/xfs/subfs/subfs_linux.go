// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package subfs provides a simple interface to create and manage a subfilesystem
// using the `open_tree` syscall. It allows for creating a new subfilesystem
// by cloning an existing filesystem tree and provides a method to close the filesystem
// when it is no longer needed.
package subfs

import (
	"fmt"
	"sync"

	"golang.org/x/sys/unix"

	xfs "github.com/siderolabs/talos/internal/pkg/xfs"
)

// Subfs represents a subfilesystem that can be created and managed.
// It uses a file descriptor to represent the mounted filesystem and provides methods
// to create and close the filesystem. The creation of the filesystem is idempotent,
// meaning it can be called multiple times without side effects.
type Subfs struct {
	createOnce  sync.Once
	createError error

	root   string
	rootfd int

	mntfd int
}

// Interface guard.
var _ xfs.Creator = (*Subfs)(nil)

// NewFrom creates a new subfs instance from root directory.
func NewFrom(root string) *Subfs {
	return &Subfs{
		root:   root,
		rootfd: unix.AT_FDCWD,
	}
}

// Create initializes the subfs filesystem and returns the file descriptor for the mounted filesystem.
// If the filesystem is already created, it returns the existing file descriptor.
// This method is idempotent, meaning it can be called multiple times without side effects.
func (subfs *Subfs) Create() (int, error) {
	subfs.createOnce.Do(func() {
		subfs.createError = subfs.new()
	})

	return subfs.mntfd, subfs.createError
}

func (subfs *Subfs) new() error {
	mntfd, err := unix.OpenTree(subfs.rootfd, subfs.root, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC)
	if err != nil {
		return fmt.Errorf("unix.OpenTree on %q failed: %w", subfs.root, err)
	}

	subfs.mntfd = mntfd

	return nil
}

// Close unmounts the subfs filesystem and closes the file descriptor.
func (subfs *Subfs) Close() error {
	return unix.Close(subfs.mntfd)
}
