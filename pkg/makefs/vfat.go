// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"context"
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
func VFAT(ctx context.Context, partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Label != "" {
		args = append(args, "-F", "32", "-n", opts.Label)
	}

	if opts.Reproducible {
		args = append(args, "--invariant")
	}

	args = append(args, partname)

	_, err := cmd.RunWithOptions(ctx, "mkfs.vfat", args)
	if err != nil {
		return err
	}

	// If source directory is specified, populate the filesystem using mtools
	if opts.SourceDirectory != "" {
		if err := populateVFAT(ctx, partname, opts.SourceDirectory); err != nil {
			return fmt.Errorf("failed to populate VFAT filesystem: %w", err)
		}
	}

	return nil
}

// populateVFAT populates a VFAT filesystem on the given partition with the
// contents of sourceDir.
func populateVFAT(ctx context.Context, partname, sourceDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory %q: %w", sourceDir, err)
	}

	for _, entry := range entries {
		switch {
		case entry.Type().IsDir():
			// copy directories
		case entry.Type().IsRegular():
			// copy regular files
		default:
			return fmt.Errorf("unsupported file type for entry %q in source directory %q", entry.Name(), sourceDir)
		}

		if _, err := cmd.RunWithOptions(
			ctx,
			"mcopy",
			[]string{
				"-s", // recursive
				"-p", // preserve attributes
				"-Q", // quit on error
				"-m", // preserve modification time
				"-i",
				partname,
				filepath.Join(sourceDir, entry.Name()),
				"::",
			},
		); err != nil {
			return err
		}
	}

	return nil
}
