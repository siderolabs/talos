// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"fmt"
	"strings"

	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/kexec"
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

// SelectUpgradeTarget flips the default boot label away from the booted one.
//
// The entry the system is running from is kept as the fallback, and the other one is overwritten,
// even if the default entry points to the other entry (e.g. an operator manually selected the
// fallback entry in the GRUB menu after a failed upgrade).
// If the booted entry is not known, it falls back to flipping the default entry.
func (c *Config) SelectUpgradeTarget(printf func(string, ...any)) error {
	if c.Booted == "" {
		return c.flip()
	}

	next, err := flipBootLabel(c.Booted)
	if err != nil {
		return err
	}

	if c.Booted != c.Default {
		printf("GRUB: booted entry %q differs from the default entry %q, keeping the booted entry as a fallback", c.Booted, c.Default)
	}

	c.Default = next
	c.Fallback = c.Booted

	return nil
}

// DetectBooted detects the entry the system is running from based on the kernel command line.
//
// GRUB passes the path to the kernel as BOOT_IMAGE, which is matched against the entries.
func (c *Config) DetectBooted(cmdline *procfs.Cmdline) {
	c.Booted = ""

	if cmdline == nil {
		return
	}

	bootImage := cmdline.Get(kexec.BootImageParam).First()
	if bootImage == nil {
		return
	}

	path := *bootImage

	// strip the GRUB device prefix, e.g. `(hd0,gpt3)/A/vmlinuz`
	if strings.HasPrefix(path, "(") {
		if idx := strings.Index(path, ")"); idx >= 0 {
			path = path[idx+1:]
		}
	}

	for _, label := range []BootLabel{BootA, BootB} {
		if entry, ok := c.Entries[label]; ok && entry.Linux == path {
			c.Booted = label

			return
		}
	}
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
