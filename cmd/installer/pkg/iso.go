// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"

	"github.com/talos-systems/go-cmd/pkg/cmd"
)

// CreateISO creates an iso by invoking the `grub-mkrescue` command.
func CreateISO(iso, dir string) (err error) {
	_, err = cmd.Run(
		"grub-mkrescue",
		"--compress=xz",
		"--output="+iso,
		dir,
	)

	if err != nil {
		return fmt.Errorf("failed to create ISO: %w", err)
	}

	return nil
}
