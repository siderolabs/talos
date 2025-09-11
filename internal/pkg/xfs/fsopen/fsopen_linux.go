// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package fsopen provides a simple interface to create and manage a filesystem
// using the Linux syscalls for filesystem operations.
package fsopen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/xfs"
	"github.com/siderolabs/talos/pkg/makefs"
)

// ErrRepairUnsupported is reported when the filesystem does not support repairs.
var ErrRepairUnsupported = errors.New("unsupported filesystem type for repair")

// FS represents a filesystem that can be created and managed.
// It holds the flags and strings used for configuration, as well as the file descriptor
// for the mounted filesystem.
type FS struct {
	fstype string
	source string

	printer func(string, ...any)

	boolParams   map[string]struct{}
	stringParams map[string][]string
	binaryParams map[string][][]byte

	mntfd    int
	mntflags int
}

// Interface guard.
var (
	_ xfs.FS = (*FS)(nil)
)

// New creates a new FS instance with the provided options.
func New(fstype string, opts ...Option) *FS {
	fs := &FS{
		fstype:       fstype,
		boolParams:   make(map[string]struct{}),
		stringParams: make(map[string][]string),
		binaryParams: make(map[string][][]byte),
	}

	for _, opt := range defaultOpts(fstype, opts...) {
		opt.set(fs)
	}

	return fs
}

// defaultOpts applies default options for filesystems.
func defaultOpts(fstype string, opts ...Option) []Option {
	if fstype == "iso9660" {
		opts = append(
			opts,
			WithBoolParameter("ro"),
		)
	}

	return opts
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

func discard(string, ...any) {}

//nolint:gocyclo
func (fs *FS) new() (err error) {
	var fsfd int

	printer := discard
	if fs.printer != nil {
		printer = fs.printer
	}

	printer("creating filesystem of type: %q", fs.fstype)

	fsfd, err = unix.Fsopen(fs.fstype, unix.FSOPEN_CLOEXEC)
	if err != nil {
		return fmt.Errorf("unix.Fsopen fstype=%q failed: %w", fs.fstype, err)
	}

	defer func() {
		if cloErr := unix.Close(fsfd); err == nil {
			err = cloErr
		} else {
			err = errors.Join(err, cloErr)
		}
	}()

	if fs.source != "" {
		printer("setting source: %q", fs.source)

		if err := unix.FsconfigSetString(fsfd, "source", fs.source); err != nil {
			return fmt.Errorf("FSCONFIG_SET_STRING failed: %w: key=%q value=%q", err, "source", fs.source)
		}
	}

	for key := range fs.boolParams {
		printer("setting boolean flag: %q", key)

		if err := unix.FsconfigSetFlag(fsfd, key); err != nil {
			return fmt.Errorf("FSCONFIG_SET_FLAG failed: %w: key=%q", err, key)
		}
	}

	for key, binary := range fs.binaryParams {
		for _, bf := range binary {
			printer("setting binary param: %q", key)

			if err := unix.FsconfigSetBinary(fsfd, key, bf); err != nil {
				return fmt.Errorf("FSCONFIG_SET_BINARY failed: %w: key=%q", err, key)
			}
		}
	}

	for key, values := range fs.stringParams {
		for _, value := range values {
			printer("setting string param: %q=%q", key, value)

			if err := unix.FsconfigSetString(fsfd, key, value); err != nil {
				return fmt.Errorf("FSCONFIG_SET_BINARY failed: %w: key=%q", err, key)
			}
		}
	}

	err = unix.FsconfigCreate(fsfd)
	if err != nil {
		return fmt.Errorf("FSCONFIG_CMD_CREATE failed: %w", err)
	}

	fs.mntflags |= unix.FSMOUNT_CLOEXEC

	fs.mntfd, err = unix.Fsmount(fsfd, fs.mntflags, 0)
	if err != nil {
		return fmt.Errorf("FSMOUNT failed: %w", err)
	}

	return nil
}

// Close closes the file descriptor.
func (fs *FS) Close() error {
	if fs.mntfd == 0 {
		return os.ErrClosed
	}

	oldmntfd := fs.mntfd
	fs.mntfd = 0

	return unix.Close(oldmntfd)
}

// Repair attempts to repair the filesystem if it is in a dirty state.
func (fs *FS) Repair() error {
	var repairFunc func(partition string) error

	switch fs.fstype {
	case makefs.FilesystemTypeEXT4:
		repairFunc = makefs.Ext4Repair
	case makefs.FilesystemTypeXFS:
		repairFunc = makefs.XFSRepair
	default:
		return fmt.Errorf("%w: %s", ErrRepairUnsupported, fs.fstype)
	}

	if err := repairFunc(fs.source); err != nil {
		return fmt.Errorf("repair %q: %w", fs.source, err)
	}

	return nil
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

// Source returns the source string used to create the filesystem.
func (fs *FS) Source() string {
	return fs.source
}

// FSType returns the filesystem type string.
func (fs *FS) FSType() string {
	return fs.fstype
}
