// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"
	"os"
	"time"

	"github.com/talos-systems/go-cmd/pkg/cmd"
)

// CreateISO creates an iso by invoking the `grub-mkrescue` command.
func CreateISO(iso, dir string) error {
	args := []string{
		"--compress=xz",
		"--output=" + iso,
		dir,
	}

	if epoch, ok, err := SourceDateEpoch(); err != nil {
		return err
	} else if ok {
		// set EFI FAT image serial number
		if err := os.Setenv("GRUB_FAT_SERIAL_NUMBER", fmt.Sprintf("%x", uint32(epoch))); err != nil {
			return err
		}

		args = append(args,
			"--",
			"-volume_date", "all_file_dates", fmt.Sprintf("=%d", epoch),
			"-volume_date", "uuid", time.Unix(epoch, 0).Format("2006010215040500"),
		)
	}

	_, err := cmd.Run("grub-mkrescue", args...)
	if err != nil {
		return fmt.Errorf("failed to create ISO: %w", err)
	}

	return nil
}
