// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package filetree

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// ChownRecursive changes file ownership recursively from the specified root.
func ChownRecursive(root string, uid, gid uint32) error {
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Sys().(*syscall.Stat_t).Uid != uid || info.Sys().(*syscall.Stat_t).Gid != gid {
			return os.Chown(path, int(uid), int(gid))
		}

		return nil
	})
}
