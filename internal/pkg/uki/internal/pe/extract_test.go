// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pe_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/pkg/uki/internal/pe"
)

func TestUKIExtract(t *testing.T) {
	srcFile := "testdata/sd-stub-amd64.efi"

	destDir := t.TempDir()

	destFile := filepath.Join(destDir, "vmlinuz.efi")

	for _, section := range []string{"linux", "initrd", "cmdline"} {
		assert.NoError(t, os.WriteFile(filepath.Join(destDir, section), []byte(section), 0o644))
	}

	assert.NoError(t, pe.AssembleNative(srcFile, destFile, []pe.Section{
		{
			Name:    ".linux",
			Path:    filepath.Join(destDir, "linux"),
			Measure: false,
			Append:  true,
		},
		{
			Name:    ".initrd",
			Path:    filepath.Join(destDir, "initrd"),
			Measure: false,
			Append:  true,
		},
		{
			Name:    ".cmdline",
			Path:    filepath.Join(destDir, "cmdline"),
			Measure: false,
			Append:  true,
		},
	}))

	ukiData, err := pe.Extract(destFile)
	assert.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, ukiData.Close())
	})

	var kernel, initrd, cmdline strings.Builder

	_, err = io.Copy(&kernel, ukiData.Kernel)
	assert.NoError(t, err)

	assert.Equal(t, "linux", kernel.String())

	_, err = io.Copy(&initrd, ukiData.Initrd)
	assert.NoError(t, err)

	assert.Equal(t, "initrd", initrd.String())

	_, err = io.Copy(&cmdline, ukiData.Cmdline)
	assert.NoError(t, err)

	assert.Equal(t, "cmdline", cmdline.String())
}
