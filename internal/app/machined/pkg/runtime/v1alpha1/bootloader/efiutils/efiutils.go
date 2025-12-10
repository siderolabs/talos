// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package efiutils provides common bootloader utils.
package efiutils

import (
	"fmt"
	"path/filepath"
)

// Name returns the standard EFI file path for the given architecture.
func Name(arch string) (string, error) {
	basePath := filepath.Join("EFI", "boot")

	switch arch {
	case "amd64":
		return filepath.Join(basePath, "BOOTX64.efi"), nil
	case "arm64":
		return filepath.Join(basePath, "BOOTAA64.efi"), nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
}
