// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"
	"log"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

const (
	// MinRAWDiskSize is the minimum size disk we can create. Used for metal images.
	MinRAWDiskSize = 1500

	// DefaultRAWDiskSize is the value we use for any non-metal images by default.
	DefaultRAWDiskSize = 8192
)

// CreateRawDisk creates a raw disk by invoking the `dd` command.
func CreateRawDisk(p runtime.Platform, diskSize int) (img string, err error) {
	img = "/tmp/disk.raw"

	// In the case that no disk size is specified, determine if we should use the min size (metal images)
	// or the default for all other images.
	if diskSize == 0 {
		if p.Name() == "metal" {
			diskSize = MinRAWDiskSize
		} else {
			diskSize = DefaultRAWDiskSize
		}
	}

	// Protect against users creating a disk that's too small
	if diskSize < MinRAWDiskSize {
		log.Printf("specified disk size too small, using minimum value of %d MB", MinRAWDiskSize)
		diskSize = MinRAWDiskSize
	}

	log.Printf("creating raw disk of size %d MB", diskSize)

	seek := fmt.Sprintf("seek=%d", diskSize)

	if _, err = cmd.Run("dd", "if=/dev/zero", "of="+img, "bs=1M", "count=0", seek); err != nil {
		return "", fmt.Errorf("failed to create RAW disk: %w", err)
	}

	return img, nil
}
