// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"
	"os"
)

const (
	// RAWDiskSize is the minimum size disk we can create.
	RAWDiskSize = 546
)

// CreateRawDisk creates a raw disk by invoking the `dd` command.
func CreateRawDisk() (img string, err error) {
	img = "/tmp/disk.raw"

	var f *os.File

	f, err = os.Create(img)
	if err != nil {
		return "", fmt.Errorf("failed to create RAW disk: %w", err)
	}

	if err = f.Truncate(RAWDiskSize * 1048576); err != nil {
		return "", fmt.Errorf("failed to truncate RAW disk: %w", err)
	}

	err = f.Close()

	return img, err
}
