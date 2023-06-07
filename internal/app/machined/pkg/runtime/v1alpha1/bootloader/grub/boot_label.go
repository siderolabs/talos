// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"fmt"
	"strings"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/bootloader"
)

// Flip flips the default boot label.
func (c *Config) Flip() error {
	if c == nil {
		return fmt.Errorf("cannot flip bootloader: %w", bootloaderNotInstalledError{})
	}

	current := c.Next

	next, err := bootloader.FlipBootLabel(c.Next)
	if err != nil {
		return err
	}

	c.Next = next
	c.Fallback = current

	return nil
}

// NextLabel returns the next bootloader label.
func (c *Config) NextLabel() string {
	// If the bootloader is not installed, return the default label.
	if c == nil {
		return string(bootloader.BootA)
	}

	return string(c.Next)
}

// PreviousLabel returns the previous bootloader label.
func (c *Config) PreviousLabel() string {
	// If the bootloader is not installed, empty.
	if c == nil {
		return ""
	}

	return string(c.Fallback)
}

// ParseBootLabel parses the given human-readable boot label to a bootloader.BootLabel.
func ParseBootLabel(name string) (bootloader.BootLabel, error) {
	switch {
	case strings.HasPrefix(name, string(bootloader.BootA)):
		return bootloader.BootA, nil
	case strings.HasPrefix(name, string(bootloader.BootB)):
		return bootloader.BootB, nil
	case strings.HasPrefix(name, "Reset"):
		return bootloader.BootReset, nil
	default:
		return "", fmt.Errorf("could not parse boot entry from name: %s", name)
	}
}
