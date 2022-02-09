// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub_test

import (
	"bytes"
	_ "embed"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/pkg/version"
)

var (
	//go:embed testdata/grub_parse_test.cfg
	grubCfg []byte

	//go:embed testdata/grub_write_test.cfg
	newConfig string
)

func TestDecode(t *testing.T) {
	conf, err := grub.Decode(grubCfg)
	assert.NoError(t, err)

	assert.Equal(t, grub.BootA, conf.Default)
	assert.Equal(t, grub.BootB, conf.Fallback)

	assert.Len(t, conf.Entries, 2)

	a := conf.Entries[grub.BootA]
	assert.Equal(t, "A - v1", a.Name)
	assert.True(t, strings.HasPrefix(a.Linux, "/A/"))
	assert.True(t, strings.HasPrefix(a.Initrd, "/A/"))
	assert.Equal(t, "cmdline A", a.Cmdline)

	b := conf.Entries[grub.BootB]
	assert.Equal(t, "B - v2", b.Name)
	assert.Equal(t, "cmdline B", b.Cmdline)
	assert.True(t, strings.HasPrefix(b.Linux, "/B/"))
	assert.True(t, strings.HasPrefix(b.Initrd, "/B/"))
}

func TestParseBootLabel(t *testing.T) {
	label, err := grub.ParseBootLabel("A - v1")
	assert.NoError(t, err)
	assert.Equal(t, grub.BootA, label)

	label, err = grub.ParseBootLabel("B - v2")
	assert.NoError(t, err)
	assert.Equal(t, grub.BootB, label)

	_, err = grub.ParseBootLabel("C - v3")
	assert.Error(t, err)
}

//nolint:errcheck
func TestWriteToFile(t *testing.T) {
	version.Name = "Test"
	version.Tag = "v0.0.1"
	version.SHA = "TEST"

	tempFile, _ := ioutil.TempFile("", "talos-test-grub-*.cfg")

	defer os.Remove(tempFile.Name())

	config := grub.NewConfig("cmdline A")

	err := config.Write(tempFile.Name())
	assert.NoError(t, err)

	written, _ := ioutil.ReadFile(tempFile.Name())
	assert.Equal(t, newConfig, string(written))
}

func TestPut(t *testing.T) {
	config := grub.NewConfig("cmdline A")
	err := config.Put(grub.BootB, "cmdline B")

	assert.NoError(t, err)

	assert.Len(t, config.Entries, 2)
	assert.Equal(t, "cmdline B", config.Entries[grub.BootB].Cmdline)

	err = config.Put(grub.BootA, "cmdline A 2")
	assert.NoError(t, err)

	assert.Equal(t, "cmdline A 2", config.Entries[grub.BootA].Cmdline)
}

//nolint:errcheck
func TestFallback(t *testing.T) {
	config := grub.NewConfig("cmdline A")
	_ = config.Put(grub.BootB, "cmdline B")

	config.Fallback = grub.BootB

	var buf bytes.Buffer
	_ = config.Encode(&buf)

	result := buf.String()

	assert.Contains(t, result, `set fallback="B - `)

	buf.Reset()

	config.Fallback = ""
	_ = config.Encode(&buf)

	result = buf.String()
	assert.NotContains(t, result, "set fallback")
}
