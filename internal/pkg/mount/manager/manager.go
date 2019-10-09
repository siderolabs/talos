/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package manager

import (
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/pkg/mount"
)

// Manager represents a management layer for a set of mountpoints.
type Manager struct {
	mountpoints *mount.Points
}

// NewManager initializes and returns a Manager.
func NewManager(mountpoints *mount.Points) *Manager {
	m := &Manager{
		mountpoints: mountpoints,
	}

	return m
}

// MountAll mounts the device(s).
func (m *Manager) MountAll() (err error) {
	iter := m.mountpoints.Iter()

	//  Mount the device(s).

	for iter.Next() {
		mountpoint := iter.Value()
		// Repair the disk's partition table.
		if mountpoint.Resize {
			if err = mountpoint.ResizePartition(); err != nil {
				return errors.Wrap(err, "resize")
			}
		}

		if err = mountpoint.Mount(); err != nil {
			return errors.Wrap(err, "mount")
		}

		// Grow the filesystem to the maximum allowed size.
		if mountpoint.Resize {
			if err = mountpoint.GrowFilesystem(); err != nil {
				return errors.Wrap(err, "grow")
			}
		}
	}

	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}

// UnmountAll unmounts the device(s).
func (m *Manager) UnmountAll() (err error) {
	iter := m.mountpoints.IterRev()
	for iter.Next() {
		mountpoint := iter.Value()
		if err = mountpoint.Unmount(); err != nil {
			return errors.Wrap(err, "unmount")
		}
	}

	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}

// MoveAll moves the device(s).
// TODO(andrewrynhard): We need to skip calling the move method on mountpoints
// that are a child of another mountpoint. The kernel will handle moving the
// child mountpoints for us.
func (m *Manager) MoveAll(prefix string) (err error) {
	iter := m.mountpoints.Iter()
	for iter.Next() {
		mountpoint := iter.Value()
		if err = mountpoint.Move(prefix); err != nil {
			return errors.Wrapf(err, "move")
		}
	}

	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}
