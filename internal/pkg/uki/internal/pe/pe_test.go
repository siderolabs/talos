// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pe_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/uki/internal/pe"
)

func assertToolsPresent(t *testing.T) {
	t.Helper()

	for _, tool := range []string{
		"objcopy",
		"objdump",
		"xxd",
	} {
		_, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("missing tool: %s", tool)
		}
	}
}

func TestAssembleNative(t *testing.T) {
	assertToolsPresent(t)

	t.Setenv("SOURCE_DATE_EPOCH", "1609459200")

	tmpDir := t.TempDir()

	outNative := filepath.Join(tmpDir, "uki-native.bin")
	outObjcopy := filepath.Join(tmpDir, "uki-objcopy.bin")

	unamePath := filepath.Join(tmpDir, "uname")
	require.NoError(t, os.WriteFile(unamePath, []byte("Talos"), 0o644))

	linuxPath := filepath.Join(tmpDir, "linux")
	require.NoError(t, os.WriteFile(linuxPath, bytes.Repeat([]byte{0xde, 0xad, 0xbe, 0xef}, 1048576), 0o644))

	sections := func() []pe.Section {
		return []pe.Section{
			{
				Name: ".text",
			},
			{
				Name:   ".uname",
				Append: true,

				Path: unamePath,
			},
			{
				Name:   ".linux",
				Append: true,

				Path: linuxPath,
			},
		}
	}

	require.NoError(t, pe.AssembleNative("testdata/linuxx64.efi.stub", outNative, sections()))

	require.NoError(t, pe.AssembleObjcopy(t.Context(), "testdata/linuxx64.efi.stub", outObjcopy, sections()))

	headersNative := dumpHeaders(t, outNative)
	headersObjcopy := dumpHeaders(t, outObjcopy)

	// we don't compute the checksums, so ignore these fields
	headersObjcopy = regexp.MustCompile(`(CheckSum\s+)[0-9a-fA-F]+`).ReplaceAllString(headersObjcopy, "${1}00000000")
	// we don't set linker version
	headersObjcopy = regexp.MustCompile(`((Major|Minor)LinkerVersion\s+)[0-9.]+`).ReplaceAllString(headersObjcopy, "${1}0")

	assert.Equal(t, headersObjcopy, headersNative)

	for _, sectionName := range []string{
		".text",
		".rodata",
		".data",
		".sbat",
		".sdmagic",
		".reloc",
		".uname",
		".linux",
	} {
		sectionObjcopy := extractSection(t, outObjcopy, sectionName)
		sectionNative := extractSection(t, outNative, sectionName)

		assert.Equal(t, sectionObjcopy, sectionNative)
	}

	if false {
		// deep binary comparison, disabled by default, as there will be some difference always
		binaryObjcopy := binaryDump(t, outObjcopy)
		binaryNative := binaryDump(t, outNative)

		assert.Equal(t, binaryObjcopy, binaryNative)
	}
}

func dumpHeaders(t *testing.T, path string) string {
	t.Helper()

	output, err := exec.CommandContext(t.Context(), "objdump", "-x", path).CombinedOutput()
	require.NoError(t, err, string(output))

	output = bytes.ReplaceAll(output, []byte(path), []byte("uki.bin"))

	return string(output)
}

func binaryDump(t *testing.T, path string) string {
	t.Helper()

	output, err := exec.CommandContext(t.Context(), "xxd", path).CombinedOutput()
	require.NoError(t, err, string(output))

	return string(output)
}

func extractSection(t *testing.T, path, section string) string {
	t.Helper()

	output, err := exec.CommandContext(t.Context(), "objdump", "-s", "--section", section, path).CombinedOutput()
	require.NoError(t, err, string(output))

	output = bytes.ReplaceAll(output, []byte(path), []byte("uki.bin"))

	return string(output)
}

func TestMultipleSections(t *testing.T) {
	assertToolsPresent(t)

	tmpDir := t.TempDir()

	unamePath := filepath.Join(tmpDir, "uname")
	require.NoError(t, os.WriteFile(unamePath, []byte("Talos"), 0o644))

	unameNewPath := filepath.Join(tmpDir, "uname-new")
	require.NoError(t, os.WriteFile(unameNewPath, []byte("Talos-new"), 0o644))

	outNative := filepath.Join(tmpDir, "uki-native.bin")

	sections := func() []pe.Section {
		return []pe.Section{
			{
				Name: ".text",
			},
			{
				Name:   ".uname",
				Append: true,
				Path:   unamePath,
			},
			{
				Name:   ".uname",
				Append: true,
				Path:   unameNewPath,
			},
		}
	}

	require.NoError(t, pe.AssembleNative("testdata/linuxx64.efi.stub", outNative, sections()))

	sectionContents := extractSection(t, outNative, ".uname")

	assert.Contains(t, sectionContents, "Talos")
	assert.Contains(t, sectionContents, "Talos-new")
}
