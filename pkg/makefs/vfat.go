// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs

import (
	"fmt"

	"github.com/talos-systems/talos/pkg/cmd"
)

// VFAT creates a VFAT filesystem on the specified partition.
func VFAT(partname string, setters ...Option) error {
	opts := NewDefaultOptions(setters...)

	args := []string{}

	if opts.Label != "" {
		args = append(args, "-F", "32", "-n", opts.Label)
	}

	args = append(args, partname)

	_, err := cmd.Run("mkfs.vfat", args...)

	return fmt.Errorf("failed to run mkfs.vfat: %w", err)
}
