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

	return err
}

// XFSRepair repairs a XFS filesystem on the specified partition.
func XFSRepair(partname, fsType string) error {
	if fsType != FilesystemTypeXFS {
		return fmt.Errorf("unsupported filesystem type: %s", fsType)
	}

	_, err := cmd.Run("xfs_repair", partname)

	return err
}

// XFS creates a XFS filesystem on the specified partition.
func XFS(partname string, setters ...Option) error {
	if partname == "" {
		return errors.New("missing path to disk")
	}

	opts := NewDefaultOptions(setters...)

	// The ftype=1 naming option is required by overlayfs.
	// The bigtime=1 metadata option enables timestamps beyond 2038.
	args := []string{"-n", "ftype=1", "-m", "bigtime=1"}

	if opts.Force {
		args = append(args, "-f")
	}

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	if opts.UnsupportedFSOption {
		args = append(args, "--unsupported")
	}

	args = append(args, partname)

	_, err := cmd.Run("mkfs.xfs", args...)

	return err
}
