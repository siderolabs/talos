// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package utils

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// TouchFiles updates mtime for all the files under root if SOURCE_DATE_EPOCH is set.
func TouchFiles(printf func(string, ...any), root string) error {
	epochInt, ok, err := SourceDateEpoch()
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	timestamp := time.Unix(epochInt, 0)

	printf("changing timestamps under %q to %s", root, timestamp)

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		return os.Chtimes(path, timestamp, timestamp)
	})
}
