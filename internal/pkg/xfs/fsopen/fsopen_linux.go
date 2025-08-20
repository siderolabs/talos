// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package fsopen provides a simple interface to create and manage a filesystem
// using the Linux syscalls for filesystem operations.
package fsopen

import (
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/unix"

	xfs "github.com/siderolabs/talos/internal/pkg/xfs"
)

// FS represents a filesystem that can be created and managed.
// It holds the flags and strings used for configuration, as well as the file descriptor
// for the mounted filesystem.
type FS struct {
	fstype string

	boolParams   []string
	stringParams map[string]string
	binaryParams map[string][]byte

	mntfd int
}

// Interface guard.
var (
	_ xfs.FS = (*FS)(nil)
)

// New creates a new FS instance with the provided options.
func New(fstype string, opts ...Option) (*FS, error) {
	fs := &FS{
		fstype:       fstype,
		boolParams:   []string{},
		stringParams: make(map[string]string),
		binaryParams: make(map[string][]byte),
	}

	for _, opt := range opts {
		if err := opt.set(fs); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return fs, nil
}

// Open initializes the filesystem and returns the file descriptor for the mounted filesystem.
// If the filesystem is already created, it returns the existing file descriptor.
// This method is idempotent, meaning it can be called multiple times without side effects.
func (fs *FS) Open() (int, error) {
	if fs.mntfd != 0 {
		return fs.mntfd, nil
	}

	err := fs.new()

	return fs.mntfd, err
}

//nolint:gocyclo
func (fs *FS) new() (err error) {
	var fsfd int

	fsfd, err = unix.Fsopen(fs.fstype, unix.FSOPEN_CLOEXEC)
	if err != nil {
		return fmt.Errorf("unix.Fsopen failed: %w", err)
	}

	defer func() {
		if cloErr := unix.Close(fsfd); err == nil {
			err = cloErr
		} else {
			err = errors.Join(err, cloErr)
		}
	}()

	for _, flag := range fs.boolParams {
		if err := unix.FsconfigSetFlag(fsfd, flag); err != nil {
			return fmt.Errorf("unix.FsconfigSetFlag failed for key %q: %w", flag, err)
		}
	}

	for key, binary := range fs.binaryParams {
		if err := unix.FsconfigSetBinary(fsfd, key, binary); err != nil {
			return fmt.Errorf("unix.FsconfigSetBinary failed for key %q: %w", key, err)
		}
	}

	for key, value := range fs.stringParams {
		if err := unix.FsconfigSetString(fsfd, key, value); err != nil {
			return fmt.Errorf("unix.FsconfigSetString failed for key %q: %w", key, err)
		}
	}

	err = unix.FsconfigCreate(fsfd)
	if err != nil {
		return fmt.Errorf("unix.FsconfigCreate failed: %w", err)
	}

	fs.mntfd, err = unix.Fsmount(fsfd, unix.FSMOUNT_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("unix.Fsmount failed: %w", err)
	}

	return nil
}

// Close closes the file descriptor.
func (fs *FS) Close() error {
	if fs.mntfd == 0 {
		return nil
	}

	oldmntfd := fs.mntfd
	fs.mntfd = 0

	return unix.Close(oldmntfd)
}

// MountAt mounts the filesystem at the specified path.
//
// EXPERIMENTAL: This function is experimental and may change in the future.
func (fs *FS) MountAt(path string) (string, error) {
	realpath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%w: %q -> %q", err, path, realpath)
	}

	if err := fs.mountAt(realpath); err != nil {
		return "", fmt.Errorf("%w: %q", err, realpath)
	}

	return realpath, nil
}

func (fs *FS) mountAt(path string) error {
	return unix.MoveMount(fs.mntfd, "", unix.AT_FDCWD, path, unix.MOVE_MOUNT_F_EMPTY_PATH)
}

// UnmountFrom unmounts the filesystem from the specified path.
//
// EXPERIMENTAL: This function is experimental and may change in the future.
func (fs *FS) UnmountFrom(path string) error {
	return unix.Unmount(path, 0)
}
