// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"github.com/siderolabs/go-cmd/pkg/cmd"
)

// ExtGrow expands an ext4 filesystem to the maximum possible.
func ExtGrow(partname string) error {
	_, err := cmd.Run("resize2fs", partname)

	return err
}

// ExtRepair repairs an ext2/3/4 filesystem on the specified partition.
func ExtRepair(partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Reproducible {
		args = append(args, "-f")
	}

	args = append(args, partname)

	_, err := cmd.Run("e2fsck", args...)

	return err
}
