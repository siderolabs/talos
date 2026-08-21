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
	t.Parallel()

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
			name:             "dirty_version_initial",
			version:          "v1.11.0-alpha.3-40-ge4c24983e-dirty",
			existingFiles:    []string{"Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty.efi"},
			expectedFileName: "Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty~1.efi",
		},
		{
			name:             "dirty_suffixed_version",
			version:          "v1.11.0-alpha.3-40-ge4c24983e-dirty",
			existingFiles:    []string{"Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty~1.efi", "Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty.efi"},
			expectedFileName: "Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty~2.efi",
		},
	} {
		t.Run(testData.name, func(t *testing.T) {
			t.Parallel()

			ukiPath, err := sdboot.GenerateNextUKIName(testData.version, testData.existingFiles)
			require.NoError(t, err)

			require.Equal(t, testData.expectedFileName, ukiPath)
		})
	}
}

func TestFindMatchingUKIFile(t *testing.T) {
	t.Parallel()

	existingFiles := []string{
		"/EFI/boot/Linux/Talos-1.10.0.efi",
		"/EFI/boot/Linux/Talos-1.10.0~1.efi",
		"/EFI/boot/Linux/talos-1.11.0.efi",
		"/EFI/boot/Linux/Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty.efi",
		"/EFI/boot/Linux/Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty~1.efi",
	}

	tests := []struct {
		existingFiles  []string
		entry          string
		expectedFile   string
		expectingFound bool
	}{
		{
			existingFiles:  existingFiles,
			entry:          "Talos-1.10.0.efi",
			expectedFile:   "Talos-1.10.0.efi",
			expectingFound: true,
		},
		{
			existingFiles:  existingFiles,
			entry:          "Talos-1.11.0.efi",
			expectedFile:   "Talos-1.11.0.efi",
			expectingFound: true,
		},
		{
			existingFiles:  existingFiles,
			entry:          "Talos-1.12.0.efi",
			expectedFile:   "",
			expectingFound: false,
		},
		{
			existingFiles:  existingFiles,
			entry:          "Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty.efi",
			expectedFile:   "Talos-v1.11.0-alpha.3-40-ge4c24983e-dirty.efi",
			expectingFound: true,
		},
		{
			entry:          "Talos-v1.11.0.efi",
			expectedFile:   "",
			expectingFound: false,
		},
		{
			entry:          "",
			expectedFile:   "",
			expectingFound: false,
		},
	}

	for _, test := range tests {
		foundFile, found := sdboot.FindMatchingUKIFile(test.existingFiles, test.entry)

		require.Equal(t, test.expectingFound, found)
		require.Equal(t, test.expectedFile, foundFile)
	}
}

func TestFindBootedUKIFile(t *testing.T) {
	t.Parallel()

	existingFiles := []string{
		"/EFI/Linux/Talos-old.efi",
		"/EFI/Linux/Talos-current.efi",
	}

	for _, test := range []struct {
		name          string
		defaultEntry  string
		selectedEntry string
		oneShotEntry  string
		rebootReason  string
		expectedEntry string
		expectedFound bool
	}{
		{
			// an operator picked a non-default entry in the sd-boot menu: that entry is the one running
			name:          "manual_selection_uses_selected_entry",
			defaultEntry:  "Talos-old.efi",
			selectedEntry: "Talos-current.efi",
			expectedEntry: "Talos-current.efi",
			expectedFound: true,
		},
		{
			// the installer just pointed the default at the new UKI, but the old one is still running
			name:          "post_upgrade_firmware_boot_uses_selected_entry",
			defaultEntry:  "Talos-current.efi",
			selectedEntry: "Talos-old.efi",
			expectedEntry: "Talos-old.efi",
			expectedFound: true,
		},
		{
			name:          "kexec_uses_installer_updated_default",
			defaultEntry:  "Talos-current.efi",
			selectedEntry: "Talos-old.efi",
			oneShotEntry:  "kexec reboot",
			rebootReason:  "reboot",
			expectedEntry: "Talos-current.efi",
			expectedFound: true,
		},
		{
			name:          "missing_selected_entry_falls_back_to_default",
			defaultEntry:  "Talos-current.efi",
			selectedEntry: "Talos-missing.efi",
			expectedEntry: "Talos-current.efi",
			expectedFound: true,
		},
		{
			name:          "missing_kexec_default_falls_back_to_selected",
			defaultEntry:  "Talos-missing.efi",
			selectedEntry: "Talos-current.efi",
			oneShotEntry:  "kexec reboot",
			rebootReason:  "reboot",
			expectedEntry: "Talos-current.efi",
			expectedFound: true,
		},
		{
			name:          "no_matching_entry",
			defaultEntry:  "Talos-missing-default.efi",
			selectedEntry: "Talos-missing-selected.efi",
			expectedFound: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			entry, found := sdboot.FindBootedUKIFile(
				existingFiles,
				test.defaultEntry,
				test.selectedEntry,
				test.oneShotEntry,
				test.rebootReason,
			)

			require.Equal(t, test.expectedFound, found)
			require.Equal(t, test.expectedEntry, entry)
		})
	}
}

func TestFindNextBootUKIFile(t *testing.T) {
	t.Parallel()

	existingFiles := []string{
		"/EFI/Linux/Talos-old.efi",
		"/EFI/Linux/Talos-current.efi",
	}

	for _, test := range []struct {
		name          string
		defaultEntry  string
		selectedEntry string
		expectedEntry string
		expectedFound bool
	}{
		{
			// the installer updated the default, kexec has to boot the new UKI and not the running one
			name:          "post_upgrade_uses_default_entry",
			defaultEntry:  "Talos-current.efi",
			selectedEntry: "Talos-old.efi",
			expectedEntry: "Talos-current.efi",
			expectedFound: true,
		},
		{
			// rollback moved the default back, kexec has to boot the previous UKI
			name:          "rollback_uses_default_entry",
			defaultEntry:  "Talos-old.efi",
			selectedEntry: "Talos-current.efi",
			expectedEntry: "Talos-old.efi",
			expectedFound: true,
		},
		{
			// booted off a disk image, the installer never ran, so there is no default yet
			name:          "missing_default_falls_back_to_selected",
			selectedEntry: "Talos-current.efi",
			expectedEntry: "Talos-current.efi",
			expectedFound: true,
		},
		{
			name:          "no_matching_entry",
			defaultEntry:  "Talos-missing-default.efi",
			selectedEntry: "Talos-missing-selected.efi",
			expectedFound: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			entry, found := sdboot.FindNextBootUKIFile(existingFiles, test.defaultEntry, test.selectedEntry)

			require.Equal(t, test.expectedFound, found)
			require.Equal(t, test.expectedEntry, entry)
		})
	}
}
