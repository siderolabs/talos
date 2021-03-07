// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"fmt"

	"github.com/talos-systems/go-cmd/pkg/cmd"
)

// XFSGrow expands a XFS filesystem to the maximum possible. The partition
// MUST be mounted, or this will fail.
func XFSGrow(partname string) error {
	_, err := cmd.Run("xfs_growfs", "-d", partname)

	return err
}

// XFS creates a XFS filesystem on the specified partition.
func XFS(partname string, setters ...Option) error {
	if partname == "" {
		return fmt.Errorf("missing path to disk")
	}

	opts := NewDefaultOptions(setters...)

	// The ftype=1 naming option is required by overlayfs.
	args := []string{"-n", "ftype=1"}

	if opts.Force {
		args = append(args, "-f")
	}

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	args = append(args, partname)

	_, err := cmd.Run("mkfs.xfs", args...)

	return err
}
