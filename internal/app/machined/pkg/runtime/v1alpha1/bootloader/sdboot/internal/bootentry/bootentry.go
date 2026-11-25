// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootentry names the UKI files on the ESP and picks the ones the sd-boot entry variables point to.
package bootentry

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// NextUKIName generates the next UKI name based on the version and existing files.
// It checks for existing files and increments the index if necessary.
func NextUKIName(version string, existingFiles []string) (string, error) {
	maxIndex := -1

	for _, file := range existingFiles {
		base := strings.TrimSuffix(filepath.Base(file), ".efi")
		if !strings.HasPrefix(base, "Talos-") {
			continue
		}

		suffix := strings.TrimPrefix(base, "Talos-")
		parts := strings.SplitN(suffix, "~", 2)

		if parts[0] != version {
			continue
		}

		if len(parts) == 1 {
			// Talos-{version}.efi format
			if maxIndex < 0 {
				maxIndex = 0
			}
		} else if len(parts) == 2 {
			// Talos-{version}+{index}.efi format
			if idx, err := strconv.Atoi(parts[1]); err == nil && idx > maxIndex {
				maxIndex = idx
			}
		}
	}

	if maxIndex >= 0 {
		return fmt.Sprintf("Talos-%s~%d.efi", version, maxIndex+1), nil
	}

	return fmt.Sprintf("Talos-%s.efi", version), nil
}

// Match returns the entry if one of the UKI files has its name (case-insensitively).
func Match(ukiFiles []string, entry string) (string, bool) {
	if slices.ContainsFunc(ukiFiles, func(file string) bool {
		return strings.EqualFold(filepath.Base(file), entry)
	}) {
		return entry, true
	}

	return "", false
}

// Booted returns the UKI the system is running from.
func Booted(ukiFiles []string, defaultEntry, selectedEntry, oneShotEntry, rebootReason string) (string, bool) {
	// A kexec reboot bypasses sd-boot, so LoaderEntrySelected can describe an older
	// boot. In that case the installer-updated default is the running entry.
	if rebootReason == "reboot" && oneShotEntry == "kexec reboot" {
		if entry, ok := Match(ukiFiles, defaultEntry); ok {
			return entry, true
		}

		return Match(ukiFiles, selectedEntry)
	}

	// For a firmware boot, LoaderEntrySelected is the entry actually running and
	// may intentionally differ from the persistent default after manual selection.
	if entry, ok := Match(ukiFiles, selectedEntry); ok {
		return entry, true
	}

	return Match(ukiFiles, defaultEntry)
}

// NextBoot returns the UKI sd-boot boots next, which is the persistent default.
//
// LoaderEntrySelected is only a fallback for a system which never ran the installer (e.g. booted
// off a disk image), as there LoaderEntryDefault is not set yet.
func NextBoot(ukiFiles []string, defaultEntry, selectedEntry string) (string, bool) {
	if entry, ok := Match(ukiFiles, defaultEntry); ok {
		return entry, true
	}

	return Match(ukiFiles, selectedEntry)
}
