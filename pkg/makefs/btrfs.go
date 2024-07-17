// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"github.com/siderolabs/go-cmd/pkg/cmd"
)

// Btrfs creates a btrfs filesystem on the specified partition.
func Btrfs(partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Label != "" {
		args = append(args, "--label", opts.Label)
	}

	args = append(args, partname)

	_, err := cmd.Run("mkfs.btrfs", args...)

	return err
}

// BtrfsGrow expands a btrfs filesystem to the maximum possible.
func BtrfsGrow(partname string) error {
	_, err := cmd.Run("btrfs", "filesystem", "resize", "max", partname)
	return err
}

// BtrfsRepair repairs a Btrfs filesystem on the specified partition.
func BtrfsRepair(partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Reproducible {
		args = append(args, "--init-csum-tree", "--init-extent-tree")
	}

	args = append(args, "--repair", partname)

	_, err := cmd.Run("btrfs", append([]string{"check"}, args...)...)

	return err
}
