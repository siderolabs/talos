// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// BindReadonly creates a common way to create a readonly bind mounted destination.
func BindReadonly(src, dst string) error {
	sourceFD, err := unix.OpenTree(unix.AT_FDCWD, src, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC)
	if err != nil {
		return fmt.Errorf("failed to opentree source %s: %w", src, err)
	}

	defer unix.Close(sourceFD) //nolint:errcheck

	if err := unix.MountSetattr(sourceFD, "", unix.AT_EMPTY_PATH, &unix.MountAttr{
		Attr_set: unix.MOUNT_ATTR_RDONLY,
	}); err != nil {
		return fmt.Errorf("failed to set mount attribute: %w", err)
	}

	if err := unix.MoveMount(sourceFD, "", unix.AT_FDCWD, dst, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("failed to move mount from %s to %s: %w", src, dst, err)
	}

	return nil
}
