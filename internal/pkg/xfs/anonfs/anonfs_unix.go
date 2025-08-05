// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package anonfs provides a simple interface to create and manage a filesystem
// using the Linux syscalls for filesystem operations.
package anonfs

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	xfs "github.com/siderolabs/talos/internal/pkg/xfs"
)

var defaultMountRoot = filepath.Join("/", "proc", "self", "fd")

// Type represents the type of filesystem to be used for the anonfs.
type Type string

// List of supported filesystems.
const (
	TypeTmpfs   Type = "tmpfs"
	TypeOverlay Type = "overlay"
)

// AnonFS represents a temporary filesystem that can be created and managed.
// It holds the flags and strings used for configuration, as well as the file descriptor
// for the mounted filesystem.
type AnonFS struct {
	createOnce  sync.Once
	createError error

	driver  Type
	flags   []string
	strings map[string]string

	mntfd int
}

// Interface guard.
var (
	_ xfs.Creator = (*AnonFS)(nil)
	_ xfs.Mounter = (*AnonFS)(nil)
)

// New creates a new AnonFS instance with the provided options.
func New(driver Type, opts ...Option) (*AnonFS, error) {
	anonfs := &AnonFS{
		driver: driver,
	}

	for _, opt := range opts {
		if err := opt.set(anonfs); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return anonfs, nil
}

// Create initializes the anonfs filesystem and returns the file descriptor for the mounted filesystem.
// If the filesystem is already created, it returns the existing file descriptor.
// This method is idempotent, meaning it can be called multiple times without side effects.
func (anonfs *AnonFS) Create() (int, error) {
	anonfs.createOnce.Do(func() {
		anonfs.createError = anonfs.new()
	})

	return anonfs.mntfd, anonfs.createError
}

func (anonfs *AnonFS) new() (err error) {
	var fsfd int

	fsfd, err = unix.Fsopen(string(anonfs.driver), unix.FSOPEN_CLOEXEC)
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

	for _, flag := range anonfs.flags {
		if err := unix.FsconfigSetFlag(fsfd, flag); err != nil {
			return fmt.Errorf("unix.FsconfigSetFlag failed for flag %q: %w", flag, err)
		}
	}

	for key, value := range anonfs.strings {
		if err := unix.FsconfigSetString(fsfd, key, value); err != nil {
			return fmt.Errorf("unix.FsconfigSetString failed for key %q: %w", key, err)
		}
	}

	err = unix.FsconfigCreate(fsfd)
	if err != nil {
		return fmt.Errorf("unix.FsconfigCreate failed: %w", err)
	}

	anonfs.mntfd, err = unix.Fsmount(fsfd, unix.FSMOUNT_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("unix.Fsmount failed: %w", err)
	}

	return nil
}

// Close closes the file descriptor.
func (anonfs *AnonFS) Close() error {
	return unix.Close(anonfs.mntfd)
}

func (anonfs *AnonFS) MountAt(path string) (string, error) {
	realpath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%w: %q -> %q", err, path, realpath)
	}

	if err := anonfs.mountAt(realpath); err != nil {
		return "", fmt.Errorf("%w: %q", err, realpath)
	}

	return realpath, nil
}

func (anonfs *AnonFS) mountAt(path string) error {
	return unix.MoveMount(anonfs.mntfd, "", unix.AT_FDCWD, path, unix.MOVE_MOUNT_F_EMPTY_PATH)
}

func (anonfs *AnonFS) UnmountFrom(path string) error {
	return unix.Unmount(path, 0)
}
