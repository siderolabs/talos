// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/template"
)

const confTemplate = `set default="{{ (index .Entries .Default).Name }}"
{{ with (index .Entries .Fallback).Name -}}
set fallback="{{ . }}"
{{- end }}
set timeout=3

insmod all_video

terminal_input console
terminal_output console

{{ range $key, $entry := .Entries -}}
menuentry "{{ $entry.Name }}" {
  set gfxmode=auto
  set gfxpayload=text
  linux {{ $entry.Linux }} {{ $entry.Cmdline }}
  initrd {{ $entry.Initrd }}
}
{{ end -}}
`

// Write the grub configuration to the given file.
func (c *Config) Write(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		return err
	}

	wr := new(bytes.Buffer)

	err := c.Encode(wr)
	if err != nil {
		return err
	}

	log.Printf("writing %s to disk", path)

	return os.WriteFile(path, wr.Bytes(), 0o600)
}

// Encode writes the grub configuration to the given writer.
func (c *Config) Encode(wr io.Writer) error {
	if err := c.validate(); err != nil {
		return err
	}

	t := template.Must(template.New("grub").Parse(confTemplate))

	return t.Execute(wr, c)
}
