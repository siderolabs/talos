// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub_test

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

var (
	//go:embed testdata/grub_parse_test.cfg
	grubCfg []byte

	//go:embed testdata/grub_write_test.cfg
	newConfig string

	//go:embed testdata/grub_write_no_reset_test.cfg
	newNoResetConfig string
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

	assert.True(t, conf.AddResetOption)
}

func TestEncodeDecode(t *testing.T) {
	config := grub.NewConfig()
	require.NoError(t, config.Put(grub.BootA, "talos.platform=metal talos.config=https://my-metadata.server/talos/config?hostname=${hostname}&mac=${mac}", "v1.2.3"))
	require.NoError(t, config.Put(grub.BootB, "talos.platform=metal talos.config=https://my-metadata.server/talos/config?uuid=${uuid}", "v1.3.4"))

	var b bytes.Buffer

	require.NoError(t, config.Encode(&b))

	t.Logf("config encoded to:\n%s", b.String())

	config2, err := grub.Decode(b.Bytes())
	require.NoError(t, err)

	assert.Equal(t, config, config2)
}

func TestParseBootLabel(t *testing.T) {
	label, err := grub.ParseBootLabel("A - v1")
	assert.NoError(t, err)
	assert.Equal(t, grub.BootA, label)

	label, err = grub.ParseBootLabel("B - v2")
	assert.NoError(t, err)
	assert.Equal(t, grub.BootB, label)

	label, err = grub.ParseBootLabel("Reset Talos installation and return to maintenance mode\n")
	assert.NoError(t, err)
	assert.Equal(t, grub.BootReset, label)

	_, err = grub.ParseBootLabel("C - v3")
	assert.Error(t, err)
}

//nolint:errcheck
func TestWrite(t *testing.T) {
	oldName := version.Name

	t.Cleanup(func() {
		version.Name = oldName
	})

	version.Name = "Test"

	tempFile, _ := os.CreateTemp(t.TempDir(), "talos-test-grub-*.cfg")

	config := grub.NewConfig()
	require.NoError(t, config.Put(grub.BootA, "cmdline A", "v0.0.1"))

	err := config.Write(tempFile.Name(), t.Logf)
	assert.NoError(t, err)

	written, _ := os.ReadFile(tempFile.Name())
	assert.Equal(t, newConfig, string(written))
}

//nolint:errcheck
func TestWriteNoReset(t *testing.T) {
	oldName := version.Name

	t.Cleanup(func() {
		version.Name = oldName
	})

	version.Name = "TestOld"

	tempFile, _ := os.CreateTemp(t.TempDir(), "talos-test-grub-*.cfg")

	config := grub.NewConfig()
	config.AddResetOption = false
	require.NoError(t, config.Put(grub.BootA, "cmdline A", "v0.0.1"))

	err := config.Write(tempFile.Name(), t.Logf)
	assert.NoError(t, err)

	written, _ := os.ReadFile(tempFile.Name())
	assert.Equal(t, newNoResetConfig, string(written))
}

func TestPut(t *testing.T) {
	config := grub.NewConfig()
	require.NoError(t, config.Put(grub.BootA, "cmdline A", "v1.2.3"))

	err := config.Put(grub.BootB, "cmdline B", "v1.0.0")

	assert.NoError(t, err)

	assert.Len(t, config.Entries, 2)
	assert.Equal(t, "cmdline B", config.Entries[grub.BootB].Cmdline)

	err = config.Put(grub.BootA, "cmdline A 2", "v1.3.4")
	assert.NoError(t, err)

	assert.Equal(t, "cmdline A 2", config.Entries[grub.BootA].Cmdline)
}

//nolint:errcheck
func TestFallback(t *testing.T) {
	config := grub.NewConfig()
	require.NoError(t, config.Put(grub.BootA, "cmdline A", "v1.0.0"))

	_ = config.Put(grub.BootB, "cmdline B", "1.2.0")

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

type bootEntry struct {
	Linux   string
	Initrd  string
	Cmdline string
}

// oldParser is the kexec parser used before the GRUB parser was rewritten.
//
// This makes sure Talos 0.14 can kexec into newly written GRUB config.
//
//nolint:gocyclo
func oldParser(r io.Reader) (*bootEntry, error) {
	scanner := bufio.NewScanner(r)

	entry := &bootEntry{}

	var (
		defaultEntry string
		currentEntry string
	)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "set default"):
			matches := regexp.MustCompile(`set default="(.*)"`).FindStringSubmatch(line)
			if len(matches) != 2 {
				return nil, fmt.Errorf("malformed default entry: %q", line)
			}

			defaultEntry = matches[1]
		case strings.HasPrefix(line, "menuentry"):
			matches := regexp.MustCompile(`menuentry "(.*)"`).FindStringSubmatch(line)
			if len(matches) != 2 {
				return nil, fmt.Errorf("malformed menuentry: %q", line)
			}

			currentEntry = matches[1]
		case strings.HasPrefix(line, "  linux "):
			if currentEntry != defaultEntry {
				continue
			}

			parts := strings.SplitN(line[8:], " ", 2)

			entry.Linux = parts[0]
			if len(parts) == 2 {
				entry.Cmdline = parts[1]
			}
		case strings.HasPrefix(line, "  initrd "):
			if currentEntry != defaultEntry {
				continue
			}

			entry.Initrd = line[9:]
		}
	}

	if entry.Linux == "" || entry.Initrd == "" {
		return nil, scanner.Err()
	}

	return entry, scanner.Err()
}

func TestBackwardsCompat(t *testing.T) {
	oldName := version.Name

	t.Cleanup(func() {
		version.Name = oldName
	})

	version.Name = "Test"

	var buf bytes.Buffer

	config := grub.NewConfig()
	require.NoError(t, config.Put(grub.BootA, "cmdline A", "v0.0.1"))
	require.NoError(t, config.Put(grub.BootB, "cmdline B", "v0.0.1"))
	config.Default = grub.BootB

	err := config.Encode(&buf)
	assert.NoError(t, err)

	entry, err := oldParser(&buf)
	require.NoError(t, err)

	assert.Equal(t, &bootEntry{
		Linux:   "/B/vmlinuz",
		Initrd:  "/B/initramfs.xz",
		Cmdline: "cmdline B",
	}, entry)
}
