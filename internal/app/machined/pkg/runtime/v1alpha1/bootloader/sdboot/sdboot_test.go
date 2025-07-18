// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sdboot_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
)

func TestGenerateNextUKIFileName(t *testing.T) {
	for _, testData := range []struct {
		name string

		version          string
		existingFiles    []string
		expectedFileName string
	}{
		{
			name:             "empty_existing_files",
			version:          "1.10.0",
			expectedFileName: "Talos-1.10.0.efi",
		},
		{
			name:             "initial_upgrade_to_same_version",
			version:          "1.10.0",
			existingFiles:    []string{"Talos-1.10.0.efi"},
			expectedFileName: "Talos-1.10.0~1.efi",
		},
		{
			name:             "second_upgrade_to_same_version",
			version:          "1.10.0",
			existingFiles:    []string{"Talos-1.10.0.efi", "Talos-1.10.0~1.efi"},
			expectedFileName: "Talos-1.10.0~2.efi",
		},
		{
			name:             "third_upgrade_to_same_version",
			version:          "1.10.0",
			existingFiles:    []string{"Talos-1.10.0~1.efi", "Talos-1.10.0~2.efi"},
			expectedFileName: "Talos-1.10.0~3.efi",
		},
		{
			name:             "upgrade_with_missing_version_in_index",
			version:          "1.10.0",
			existingFiles:    []string{"Talos-1.10.0~1.efi", "Talos-1.10.0~3.efi"},
			expectedFileName: "Talos-1.10.0~4.efi",
		},
		{
			name:             "upgrade_with_non-suffixed_file",
			version:          "1.10.0",
			existingFiles:    []string{"Talos-1.10.0.efi", "Talos-1.10.0~2.efi"},
			expectedFileName: "Talos-1.10.0~3.efi",
		},
		{
			name:             "direct_upgrade_to_different_version",
			version:          "1.11.0",
			existingFiles:    []string{"Talos-1.10.0.efi"},
			expectedFileName: "Talos-1.11.0.efi",
		},
		{
			name:             "direct_upgrade_to_different_version_with_different_files",
			version:          "1.11.0",
			existingFiles:    []string{"Talos-1.10.0.efi", "Talos-1.10.0~1.efi"},
			expectedFileName: "Talos-1.11.0.efi",
		},
		{
			name:             "downgrade",
			version:          "1.10.0",
			existingFiles:    []string{"Talos-1.10.0.efi", "Talos-1.11.0.efi"},
			expectedFileName: "Talos-1.10.0~1.efi",
		},
		{
			name:             "downgrade_with_suffixed_version",
			version:          "1.10.0",
			existingFiles:    []string{"Talos-1.10.0~1.efi", "Talos-1.11.0.efi"},
			expectedFileName: "Talos-1.10.0~2.efi",
		},
		{
			name:             "foo",
			version:          "v1.11.0-alpha.3-40-ge4c24983e-dirty",
			existingFiles:    []string{"Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty.efi"},
			expectedFileName: "Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty~1.efi",
		},
		{
			name:             "fobaro",
			version:          "v1.11.0-alpha.3-40-ge4c24983e-dirty",
			existingFiles:    []string{"Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty~1.efi", "Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty.efi"},
			expectedFileName: "Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty~2.efi",
		},
	} {
		t.Run(testData.name, func(t *testing.T) {
			ukiPath, err := sdboot.GenerateNextUKIName(testData.version, testData.existingFiles)
			require.NoError(t, err)

			require.Equal(t, testData.expectedFileName, ukiPath)
		})
	}
}
