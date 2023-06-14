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
func (c *Config) flip() error {
	if _, exists := c.Entries[c.Default]; !exists {
		return nil
	}

	current := c.Default

	next, err := bootloader.FlipBootLabel(c.Default)
	if err != nil {
		return err
	}

	c.Default = next
	c.Fallback = current

	return nil
}

// PreviousLabel returns the previous bootloader label.
func (c *Config) PreviousLabel() string {
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
