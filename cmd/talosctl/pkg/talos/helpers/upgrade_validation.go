// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
)

// ValidateUpgradeTransition checks if the upgrade is safe.
func ValidateUpgradeTransition(current *MachineContext, targetSchematic string, targetSecureBoot bool, targetPlatform string, targetVersion string) error {
	var warnings []string
	var errors []string

	// Validate version transitions
	if targetVersion != "" && current.Version != "" {
		if err := validateVersionTransition(current.Version, targetVersion, &warnings, &errors); err != nil {
			return err
		}
	}

	// Validate schematic format
	if targetSchematic != "" && !isValidSchematicID(targetSchematic) {
		errors = append(errors, fmt.Sprintf("Invalid schematic ID: %s (must be 64 hex chars)", targetSchematic))
	}

	// Warn on schematic changes
	if targetSchematic != current.Schematic {
		if current.Schematic != "" && targetSchematic != "" {
			warnings = append(warnings, fmt.Sprintf("Schematic changing: %s → %s", current.Schematic, targetSchematic))
		} else if current.Schematic != "" {
			warnings = append(warnings, "Removing all system extensions")
		} else if targetSchematic != "" {
			warnings = append(warnings, fmt.Sprintf("Adding schematic: %s", targetSchematic))
		}
	}

	//Prevent secure boot → non-secure boot
	if current.SecureBoot && !targetSecureBoot {
		errors = append(errors, "Cannot disable secure boot (bootloader incompatibility will prevent boot)")
	}

	// Warn when enabling secure boot
	if !current.SecureBoot && targetSecureBoot {
		warnings = append(warnings, "Enabling secure boot (ensure firmware is configured)")
	}

	// Prevent platform changes
	if targetPlatform != current.Platform && current.Platform != "" && targetPlatform != "" {
		errors = append(errors, fmt.Sprintf("Platform changes not supported: %s → %s", current.Platform, targetPlatform))
	}

	// Print warnings
	if len(warnings) > 0 {
		fmt.Println("\nUpgrade Warnings:")
		for _, w := range warnings {
			fmt.Println("  " + w)
		}
		fmt.Println()
	}

	// Return errors
	if len(errors) > 0 {
		fmt.Println("\nUpgrade Validation Errors:")
		for _, e := range errors {
			fmt.Println("  " + e)
		}
		fmt.Println("\nUse --force to override")
		return fmt.Errorf("upgrade validation failed")
	}

	return nil
}

// validateVersionTransition checks semantic versioning rules.
func validateVersionTransition(currentVer, targetVer string, warnings, errors *[]string) error {
	currentVer = strings.TrimPrefix(currentVer, "v")
	targetVer = strings.TrimPrefix(targetVer, "v")

	if currentVer == "" {
		return nil
	}

	current, err := semver.Parse(currentVer)
	if err != nil {
		return nil
	}

	target, err := semver.Parse(targetVer)
	if err != nil {
		return fmt.Errorf("invalid target version '%s'", targetVer)
	}

	// Warn on downgrades
	if target.LT(current) {
		*warnings = append(*warnings, fmt.Sprintf("Downgrade detected: %s → %s (not recommended)", currentVer, targetVer))
	}

	// Warn on version skips
	if target.Major == current.Major && target.Minor > current.Minor+1 {
		*warnings = append(*warnings, fmt.Sprintf("Skipping minor versions: %s → %s", currentVer, targetVer))
	}

	// Warn on pre-release
	if len(target.Pre) > 0 {
		*warnings = append(*warnings, fmt.Sprintf("Upgrading to pre-release version: %s", targetVer))
	}

	return nil
}

// isValidSchematicID checks if a string is a valid 64-character hex schematic ID.
func isValidSchematicID(s string) bool {
	if len(s) != 64 {
		return false
	}

	for _, ch := range s {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			return false
		}
	}

	return true
}

