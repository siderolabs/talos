// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package provision

import (
	"os"

	"golang.org/x/sys/unix"
)

const defaultContainerShmSize = 64 * 1024 * 1024 // 64MiB

func (s *State) isDevShmAvailable() bool {
	// check if /dev/shm exists
	if _, err := os.Stat("/dev/shm"); err != nil {
		return false
	}

	// get /dev/shm stats
	var stat unix.Statfs_t

	if err := unix.Statfs("/dev/shm", &stat); err != nil {
		return false
	}

	// check if is tmpfs
	if stat.Type != unix.TMPFS_MAGIC {
		return false
	}

	// check if /dev/shm has potentially enough space
	if stat.Blocks*uint64(stat.Bsize) <= defaultContainerShmSize {
		return false
	}

	return true
}
