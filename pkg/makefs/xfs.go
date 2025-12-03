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
	// FilesystemTypeXFS is the filesystem type for XFS.
	FilesystemTypeXFS = "xfs"
)

// XFSGrow expands a XFS filesystem to the maximum possible. The partition
// MUST be mounted, or this will fail.
func XFSGrow(partname string) error {
	_, err := cmd.Run("xfs_growfs", "-d", partname)
	if err != nil {
		return fmt.Errorf("failed to grow XFS filesystem: %w", err)
	}

	return err
}

// XFSRepair repairs a XFS filesystem on the specified partition.
func XFSRepair(partname string) error {
	_, err := cmd.Run("xfs_repair", partname)
	if err != nil {
		return fmt.Errorf("error repairing XFS filesystem: %w", err)
	}

	return nil
}

// XFS creates a XFS filesystem on the specified partition.
func XFS(partname string, setters ...Option) error {
	if partname == "" {
		return errors.New("missing path to disk")
	}

	opts := NewDefaultOptions(setters...)

	// The ftype=1 naming option is required by overlayfs.
	args := []string{"-n", "ftype=1"}

	if opts.ConfigFile != "" {
		args = append(args, "-c", fmt.Sprintf("options=%s", opts.ConfigFile))
	}

	if opts.Force {
		args = append(args, "-f")
	}

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	if opts.UnsupportedFSOption {
		args = append(args, "--unsupported")
	}

	if opts.SourceDirectory != "" {
		args = append(args, "-p", opts.SourceDirectory)
	}

	args = append(args, partname)

	_, err := cmd.Run("mkfs.xfs", args...)

	return err
}
