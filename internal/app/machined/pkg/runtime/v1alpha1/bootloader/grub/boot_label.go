// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"fmt"
	"strings"
)

// flipBootLabel flips the boot label.
func flipBootLabel(e BootLabel) (BootLabel, error) {
	switch e {
	case BootA:
		return BootB, nil
	case BootB:
		return BootA, nil
	case BootReset:
		fallthrough
	default:
		return "", fmt.Errorf("invalid entry: %s", e)
	}
}

// Flip flips the default boot label.
func (c *Config) flip() error {
	if _, exists := c.Entries[c.Default]; !exists {
		return nil
	}

	current := c.Default

	next, err := flipBootLabel(c.Default)
	if err != nil {
		return err
	}

	c.Default = next
	c.Fallback = current

	return nil
}

// ParseBootLabel parses the given human-readable boot label to a BootLabel.
func ParseBootLabel(name string) (BootLabel, error) {
	switch {
	case strings.HasPrefix(name, string(BootA)):
		return BootA, nil
	case strings.HasPrefix(name, string(BootB)):
		return BootB, nil
	case strings.HasPrefix(name, "Reset"):
		return BootReset, nil
	default:
		return "", fmt.Errorf("could not parse boot entry from name: %s", name)
	}
}
