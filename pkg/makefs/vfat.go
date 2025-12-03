// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

const (
	// FilesystemTypeVFAT is the filesystem type for VFAT.
	FilesystemTypeVFAT = "vfat"
)

// VFAT creates a VFAT filesystem on the specified partition.
func VFAT(partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Label != "" {
		args = append(args, "-F", "32", "-n", opts.Label)
	}

	if opts.Reproducible {
		args = append(args, "--invariant")
	}

	args = append(args, partname)

	_, err := cmd.Run("mkfs.vfat", args...)
	if err != nil {
		return err
	}

	// If source directory is specified, populate the filesystem using mtools
	if opts.SourceDirectory != "" {
		if err := populateVFAT(partname, opts.SourceDirectory); err != nil {
			return fmt.Errorf("failed to populate VFAT filesystem: %w", err)
		}
	}

	return nil
}

// populateVFAT copies files from sourceDir to the VFAT filesystem using mcopy.
func populateVFAT(partname, sourceDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Build mcopy arguments
	args := []string{
		"-s", // recursive
		"-p", // preserve attributes
		"-Q", // quit on error
		"-m", // preserve modification time
	}

	args = append(args, "-i", partname)

	// Add all entries from source directory
	for _, entry := range entries {
		args = append(args, filepath.Join(sourceDir, entry.Name()))
	}

	// Destination is the root of the VFAT filesystem
	args = append(args, "::")

	if _, err := cmd.Run("mcopy", args...); err != nil {
		return err
	}

	return nil
}
