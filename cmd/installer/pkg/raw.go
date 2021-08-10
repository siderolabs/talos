// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"

	"github.com/talos-systems/go-cmd/pkg/cmd"
)

const (
	// RAWDiskSize is the minimum size disk we can create.
	RAWDiskSize = 546
)

// CreateRawDisk creates a raw disk by invoking the `dd` command.
func CreateRawDisk() (img string, err error) {
	img = "/tmp/disk.raw"

	seek := fmt.Sprintf("seek=%d", RAWDiskSize)

	if _, err = cmd.Run("dd", "if=/dev/zero", "of="+img, "bs=1M", "count=0", seek); err != nil {
		return "", fmt.Errorf("failed to create RAW disk: %w", err)
	}

	return img, nil
}
