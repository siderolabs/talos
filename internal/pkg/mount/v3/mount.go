// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mount handles filesystem mount operations.
package mount

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// bindHardenAttr is the baseline attribute set every read-only bind mount
// inherits: read-only, no setuid escalation, no device nodes (per
// siderolabs/talos#11946 — device nodes belong only in /dev and /dev/pts).
const bindHardenAttr = unix.MOUNT_ATTR_RDONLY | unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NODEV

// BindReadonly creates a common way to create a readonly bind mounted destination.
func BindReadonly(src, dst string) error {
	sourceFD, err := unix.OpenTree(unix.AT_FDCWD, src, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC)
	if err != nil {
		return fmt.Errorf("failed to opentree source %s: %w", src, err)
	}

	defer unix.Close(sourceFD) //nolint:errcheck

	if err := unix.MountSetattr(sourceFD, "", unix.AT_EMPTY_PATH, &unix.MountAttr{
		Attr_set: bindHardenAttr,
	}); err != nil {
		return fmt.Errorf("failed to set mount attribute: %w", err)
	}

	if err := unix.MoveMount(sourceFD, "", unix.AT_FDCWD, dst, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("failed to move mount from %s to %s: %w", src, dst, err)
	}

	return nil
}

// BindReadonlyFd creates a common way to create a readonly bind mounted destination.
func BindReadonlyFd(dfd int, dst string) error {
	sourceFD, err := unix.OpenTree(dfd, "", unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC|unix.AT_EMPTY_PATH)
	if err != nil {
		return fmt.Errorf("failed to opentree: %w", err)
	}

	defer unix.Close(sourceFD) //nolint:errcheck

	if err := unix.MountSetattr(sourceFD, "", unix.AT_EMPTY_PATH, &unix.MountAttr{
		Attr_set: bindHardenAttr,
	}); err != nil {
		return fmt.Errorf("failed to set mount attribute: %w", err)
	}

	if err := unix.MoveMount(sourceFD, "", unix.AT_FDCWD, dst, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("failed to move mount to %s: %w", dst, err)
	}

	return nil
}
