// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

// Package opendir provides a filesystem rooted at an existing directory.
package opendir

import (
	"context"
	"fmt"

	"golang.org/x/sys/unix"

	xfs "github.com/siderolabs/talos/pkg/xfs"
)

// FS represents a filesystem rooted at an existing directory.
type FS struct {
	dirfd   int
	name    string
	resolve uint64

	fd int
}

// Interface guard.
var _ xfs.FS = (*FS)(nil)

// NewFromPath creates a new FS rooted at the directory at path.
func NewFromPath(path string) *FS {
	return &FS{
		dirfd: unix.AT_FDCWD,
		name:  path,
	}
}

// NewBeneath creates a new FS rooted at the directory name beneath dirfd.
//
// Resolving name fails with EXDEV if it leaves dirfd, through ".." or a symlink.
func NewBeneath(dirfd int, name string) *FS {
	return &FS{
		dirfd:   dirfd,
		name:    name,
		resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS,
	}
}

// Open opens the directory and returns its file descriptor.
func (fs *FS) Open() (int, error) {
	if fs.fd != 0 {
		return fs.fd, nil
	}

	fd, err := unix.Openat2(fs.dirfd, fs.name, &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: fs.resolve,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to open directory %q: %w", fs.name, err)
	}

	fs.fd = fd

	return fs.fd, nil
}

// Close closes the directory file descriptor.
func (fs *FS) Close() error {
	if fs.fd == 0 {
		return nil
	}

	fd := fs.fd
	fs.fd = 0

	return unix.Close(fd)
}

// Repair is a no-op for a directory.
func (fs *FS) Repair(context.Context) error {
	return nil
}

// Source returns the name of the directory.
func (fs *FS) Source() string {
	return fs.name
}

// FSType returns an empty string, as the directory isn't a filesystem of its own.
func (fs *FS) FSType() string {
	return ""
}
