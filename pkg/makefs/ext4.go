// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/pkg/imager/utils"
)

// Ext4 creates a ext4 filesystem on the specified partition.
func Ext4(partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	var args []string

	if opts.Label != "" {
		args = append(args, "-L", opts.Label)
	}

	if opts.Force {
		args = append(args, "-F")
	}

	if opts.Reproducible {
		if epoch, ok, err := utils.SourceDateEpoch(); err != nil {
			return err
		} else if ok {
			// ref: https://gitlab.archlinux.org/archlinux/archiso/-/merge_requests/202/diffs
			detUUID := uuid.NewSHA1(uuid.MustParse("93a870ff-8565-4cf3-a67b-f47299271a96"), []byte(fmt.Sprintf("%d ext4 hash seed", epoch)))

			args = append(args, "-U", detUUID.String())
			args = append(args, "-E", fmt.Sprintf("hash_seed=%s", detUUID.String()))
		}
	}

	args = append(args, partname)

	_, err := cmd.Run("mkfs.ext4", args...)

	return err
}

// Ext4Resize expands a ext4 filesystem to the maximum possible.
func Ext4Resize(partname string) error {
	_, err := cmd.Run("resize2fs", partname)

	return err
}
