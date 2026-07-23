// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package miniprocfs

import (
	"os"
	"strconv"
	"syscall"
)

// ReadExeIdentity returns the device and inode of the executable backing the process.
//
// It stats /proc/<pid>/exe, which follows the magic symlink to the real backing file, so the
// returned identity is stable regardless of the path (or hardlink name) used to exec it.
func ReadExeIdentity(pid int32) (dev, ino uint64, ok bool) {
	fi, err := os.Stat("/proc/" + strconv.Itoa(int(pid)) + "/exe")
	if err != nil {
		return 0, 0, false
	}

	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}

	return st.Dev, st.Ino, true
}
