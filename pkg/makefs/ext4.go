// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"errors"
	"fmt"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

const (
	// FilesystemTypeEXT4 is the filesystem type for EXT4.
	FilesystemTypeEXT4 = "ext4"
)

// Ext4 creates a ext4 filesystem on the specified partition.
func Ext4(partname string, setters ...Option) error {
	if partname == "" {
		return errors.New("missing path to disk")
	}

	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	if opts.Force {
		args = append(args, "-F")
	}

	if opts.Reproducible {
		if opts.Label == "" {
			return errors.New("label must be set for reproducible ext4 filesystem")
		}

		partitionGUID := GUIDFromLabel(opts.Label)

		args = append(args, "-U", partitionGUID.String())
		args = append(args, "-E", fmt.Sprintf("hash_seed=%s", partitionGUID.String()))
	}

	if opts.SourceDirectory != "" {
		args = append(args, "-d", opts.SourceDirectory)
	}

	args = append(args, partname)

	opts.Printf("creating ext4 filesystem on %s with args: %v", partname, args)

	_, err := cmd.Run("mkfs.ext4", args...)

	return err
}

// Ext4Resize expands a ext4 filesystem to the maximum possible.
func Ext4Resize(partname string) error {
	// resizing the filesystem requires a check first
	if err := Ext4Repair(partname); err != nil {
		return fmt.Errorf("failed to repair before growing ext4 filesystem: %w", err)
	}

	_, err := cmd.Run("resize2fs", partname)
	if err != nil {
		return fmt.Errorf("failed to grow ext4 filesystem: %w", err)
	}

	return nil
}

// Ext4Repair repairs a ext4 filesystem.
func Ext4Repair(partname string) error {
	_, err := cmd.Run("e2fsck", "-f", "-p", partname)
	if err != nil {
		return fmt.Errorf("failed to repair ext4 filesystem: %w", err)
	}

	return nil
}
