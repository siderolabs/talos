// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Write the grub configuration to the given file.
func (c *Config) Write(path string, printf func(string, ...any)) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	wr := new(bytes.Buffer)

	err := c.Encode(wr)
	if err != nil {
		return err
	}

	printf("writing %s to disk", path)

	return os.WriteFile(path, wr.Bytes(), 0o600)
}

// Encode writes the grub configuration to the given writer.
func (c *Config) Encode(wr io.Writer) error {
	if err := c.validate(); err != nil {
		return err
	}

	fmt.Fprintf(wr, "set default=\"%s\"\n", c.Entries[c.Default].Name)

	if fallback, ok := c.Entries[c.Fallback]; ok {
		fmt.Fprintf(wr, "set fallback=\"%s\"\n", fallback.Name)
	}

	fmt.Fprint(wr, `
set timeout=3

insmod all_video

terminal_input console
terminal_output console

`)

	for _, entry := range c.Entries {
		fmt.Fprintf(wr, `menuentry "%s" {
  set gfxmode=auto
  set gfxpayload=text
  linux %s %s
  initrd %s
}
`, entry.Name, entry.Linux, Quote(entry.Cmdline), entry.Initrd)
	}

	if c.AddResetOption {
		defaultEntry := c.Entries[c.Default]

		fmt.Fprintf(wr, `menuentry "Reset Talos installation and return to maintenance mode" {
  set gfxmode=auto
  set gfxpayload=text
  linux %s %s talos.experimental.wipe=system:EPHEMERAL,STATE
  initrd %s
}
`, defaultEntry.Linux, Quote(defaultEntry.Cmdline), defaultEntry.Initrd)
	}

	return nil
}
