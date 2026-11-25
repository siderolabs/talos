// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

// Package detach removes mounts from the sandbox mount namespace.
package detach

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Mount detaches the mount at path from the mount namespace of the calling
// thread, leaving the directory it covers. It reports false if path is not a
// mount root, e.g. it was detached already.
//
// An unmount propagates to the peers of the parent mount, and the mounts under
// path propagate theirs to its own peers. The parent is made a slave (and shared
// again, for the copies made from this namespace) and the mounts at path are made
// private, so that detaching them leaves the namespace this one was copied from alone.
//
// The directory left behind must be empty: a path which was never mounted fails
// instead of leaving its contents in place.
func Mount(path string) (bool, error) {
	root, err := isMountRoot(path)
	if err != nil {
		return false, err
	}

	if root {
		if err = detachMount(path); err != nil {
			return false, err
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}

	if len(entries) > 0 {
		return false, fmt.Errorf("%s is not empty after detaching it", path)
	}

	return root, nil
}

func detachMount(path string) error {
	parent, err := mountRootOf(filepath.Dir(path))
	if err != nil {
		return err
	}

	for _, propagation := range []uint64{unix.MS_SLAVE, unix.MS_SHARED} {
		if err = unix.MountSetattr(unix.AT_FDCWD, parent, 0, &unix.MountAttr{Propagation: propagation}); err != nil {
			return fmt.Errorf("set propagation of %s: %w", parent, err)
		}
	}

	if err = unix.MountSetattr(unix.AT_FDCWD, path, unix.AT_RECURSIVE, &unix.MountAttr{Propagation: unix.MS_PRIVATE}); err != nil {
		return fmt.Errorf("make %s private: %w", path, err)
	}

	if err = unix.Unmount(path, unix.MNT_DETACH); err != nil {
		return fmt.Errorf("detach %s: %w", path, err)
	}

	return nil
}

// mountRootOf returns the root of the mount which holds path: mount_setattr takes a mount root only.
func mountRootOf(path string) (string, error) {
	for {
		root, err := isMountRoot(path)
		if err != nil || root || path == "/" {
			return path, err
		}

		path = filepath.Dir(path)
	}
}

func isMountRoot(path string) (bool, error) {
	var stx unix.Statx_t

	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, 0, &stx); err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}

	if stx.Attributes_mask&unix.STATX_ATTR_MOUNT_ROOT == 0 {
		return false, fmt.Errorf("stat %s: no mount root attribute", path)
	}

	return stx.Attributes&unix.STATX_ATTR_MOUNT_ROOT != 0, nil
}
