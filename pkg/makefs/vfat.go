// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/filesystem"
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

	_, err := cmd.RunContext(ctx, "mkfs.vfat", args...)
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

// populateVFAT populates a VFAT filesystem on the given partition with the
// contents of sourceDir.
//
//nolint:gocyclo
func populateVFAT(partname, sourceDir string) error {
	bk, err := file.OpenFromPath(partname, false)
	if err != nil {
		return fmt.Errorf("failed to open partition %q: %w", partname, err)
	}

	defer bk.Close() //nolint:errcheck

	diskInfo, err := diskfs.OpenBackend(bk, diskfs.WithOpenMode(diskfs.ReadWrite))
	if err != nil {
		return fmt.Errorf("failed to open disk backend for partition %q: %w", partname, err)
	}

	defer diskInfo.Close() //nolint:errcheck

	dfs, err := diskInfo.GetFilesystem(0)
	if err != nil {
		return fmt.Errorf("failed to get filesystem for partition %q: %w", partname, err)
	}

	defer dfs.Close() //nolint:errcheck

	if err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("error walking through source directory %q: %w", sourceDir, walkErr)
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %q: %w", path, err)
		}

		fsPath := filepath.Join("/", relPath)

		if info.IsDir() {
			if relPath == "." {
				return nil
			}

			if err := dfs.Mkdir(fsPath); err != nil {
				return fmt.Errorf("failed to create directory %q in VFAT filesystem: %w", relPath, err)
			}

			return nil
		}

		return createFATFile(path, fsPath, dfs)
	}); err != nil {
		return err
	}

	return nil
}

func createFATFile(srcPath, destPath string, dfs filesystem.FileSystem) error {
	srcFile, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", srcPath, err)
	}

	destFile, err := dfs.OpenFile(destPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return fmt.Errorf("failed to open destination file %q in FAT filesystem: %w", destPath, err)
	}

	defer destFile.Close() //nolint:errcheck

	if _, err := destFile.Write(srcFile); err != nil {
		return fmt.Errorf("failed to write to destination file %q in FAT filesystem: %w", destPath, err)
	}

	return nil
}
