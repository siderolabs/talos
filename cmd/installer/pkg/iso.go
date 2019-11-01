// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"

	"github.com/talos-systems/talos/pkg/cmd"
)

// Mkisofs creates an iso by invoking the `mkisofs` command.
func Mkisofs(iso, dir string) (err error) {
	_, err = cmd.Run(
		"mkisofs",
		"-V", "TALOS",
		"-o", iso,
		"-r",
		"-b", "isolinux/isolinux.bin",
		"-c", "isolinux/boot.cat",
		"-no-emul-boot",
		"-boot-load-size",
		"4",
		"-boot-info-table",
		dir,
	)

	if err != nil {
		return fmt.Errorf("failed to create ISO: %w", err)
	}

	return nil
}

// Isohybrid creates a hybrid iso by invoking the `isohybrid` command.
func Isohybrid(iso string) (err error) {
	if _, err = cmd.Run("isohybrid", iso); err != nil {
		return fmt.Errorf("failed to create hybrid ISO: %w", err)
	}

	return nil
}
