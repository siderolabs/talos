// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package xfs

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
func (root *UnixRoot) RepairFS(ctx context.Context) error {
	return root.FS.Repair(ctx)
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

// resolveBeneath keeps path resolution inside the root: absolute symlinks and
// ".." leaving the root fail with EXDEV.
const resolveBeneath = unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS

// Mkdir creates a new directory in the root filesystem with the specified name and permissions.
func (root *UnixRoot) Mkdir(name string, perm os.FileMode) error {
	dirfd, base, err := root.parent(name)
	if err != nil {
		return err
	}

	defer unix.Close(dirfd) //nolint:errcheck

	return unix.Mkdirat(dirfd, base, uint32(perm))
}

// Open opens a file in the root filesystem with the specified name in read-only mode.
func (root *UnixRoot) Open(name string) (fs.File, error) {
	return root.OpenFile(name, unix.O_RDONLY, 0)
}

// OpenFile opens a file in the root filesystem with the specified name, flags, and permissions.
func (root *UnixRoot) OpenFile(name string, flags int, perm os.FileMode) (File, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}

	how := unix.OpenHow{
		Flags:   uint64(flags | unix.O_CLOEXEC),
		Resolve: resolveBeneath,
	}

	// openat2 rejects a mode unless the file may be created
	if flags&unix.O_CREAT != 0 || flags&unix.O_TMPFILE == unix.O_TMPFILE {
		how.Mode = uint64(perm.Perm())
	}

	fd, err := unix.Openat2(root.mntfd, name, &how)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), name), nil
}

// Remove removes a file or directory from the root filesystem.
//
// A symlink is removed itself, not its target.
func (root *UnixRoot) Remove(name string) error {
	dirfd, base, err := root.parent(name)
	if err != nil {
		return err
	}

	defer unix.Close(dirfd) //nolint:errcheck

	err = unix.Unlinkat(dirfd, base, 0)
	if errors.Is(err, unix.EISDIR) {
		err = unix.Unlinkat(dirfd, base, unix.AT_REMOVEDIR)
	}

	return err
}

// Rename renames a file or directory in the root filesystem from old to new.
func (root *UnixRoot) Rename(oldname, newname string) error {
	olddirfd, oldbase, err := root.parent(oldname)
	if err != nil {
		return err
	}

	defer unix.Close(olddirfd) //nolint:errcheck

	newdirfd, newbase, err := root.parent(newname)
	if err != nil {
		return err
	}

	defer unix.Close(newdirfd) //nolint:errcheck

	return unix.Renameat(olddirfd, oldbase, newdirfd, newbase)
}

// parent opens the directory holding name beneath the root and returns it with
// the last path element, which *at syscalls do not follow.
func (root *UnixRoot) parent(name string) (int, string, error) {
	name, err := cleanName(name)
	if err != nil {
		return -1, "", err
	}

	if !filepath.IsLocal(name) {
		return -1, "", fmt.Errorf("%q escapes the root: %w", name, unix.EXDEV)
	}

	dir, base := filepath.Split(name)

	dirfd, err := unix.Openat2(root.mntfd, cmp.Or(dir, "."), &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: resolveBeneath,
	})
	if err != nil {
		return -1, "", err
	}

	return dirfd, base, nil
}

// cleanName rejects names that are not clean: with RESOLVE_BENEATH, an inner ".."
// fails with EAGAIN whenever any rename or mount on the system races the lookup.
func cleanName(name string) (string, error) {
	name = strings.TrimLeft(name, "/")
	if name != filepath.Clean(name) {
		return "", fmt.Errorf("%q is not a clean path: %w", name, unix.EINVAL)
	}

	return name, nil
}

// Source returns the source of the underlying filesystem.
func (root *UnixRoot) Source() string {
	return root.FS.Source()
}

// FSType returns the type of the underlying filesystem.
func (root *UnixRoot) FSType() string {
	return root.FS.FSType()
}
