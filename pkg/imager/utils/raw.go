// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package utils

import (
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/dustin/go-humanize"
)

// CreateRawDisk creates a raw disk image of the specified size.
func CreateRawDisk(path string, diskSize int64) error {
	log.Printf("creating raw disk of size %s", humanize.Bytes(uint64(diskSize)))

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create raw disk: %w", err)
	}

	defer f.Close() //nolint:errcheck

	if err = f.Truncate(diskSize); err != nil {
		return fmt.Errorf("failed to create raw disk: %w", err)
	}

	if err = syscall.Fallocate(int(f.Fd()), 0, 0, diskSize); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to preallocate disk space for %q (size %d): %s", path, diskSize, err)
	}

	return f.Close()
}
