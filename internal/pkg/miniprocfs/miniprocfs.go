// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package miniprocfs contains optimized small interface to access /proc filesystem.
package miniprocfs

import (
	"os"
	"strconv"
)

// ReadMountNamespace returns the mount namespace of the process.
func ReadMountNamespace(pid int32) (string, bool) {
	nsPath := "/proc/" + strconv.Itoa(int(pid)) + "/ns/mnt"

	ns, err := os.Readlink(nsPath)
	if err != nil {
		return "", false
	}

	return ns, true
}
