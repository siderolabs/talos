// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package iso_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/iso"
)

func TestVolumeID(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in string

		out string
	}{
		{
			in: "Talos-v1.7.6",

			out: "TALOS_V1_7_6",
		},
		{
			in: "Talos-v1.7.6-beta.0",

			out: "TALOS_V1_7_6_BETA_0",
		},
	} {
		t.Run(test.in, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.out, iso.VolumeID(test.in))
		})
	}
}

func TestCreateGRUBPlatform(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		arch    string
		version string

		expectedDirectory string
	}{
		{
			name:    "amd64 BIOS only",
			arch:    "amd64",
			version: "v1.12.0",

			expectedDirectory: "--directory=/usr/lib/grub/i386-pc",
		},
		{
			name:    "arm64 EFI only",
			arch:    "arm64",
			version: "v1.12.0",

			expectedDirectory: "--directory=/usr/lib/grub/arm64-efi",
		},
		{
			// legacy GRUB ISOs cover both BIOS and UEFI boot, grub-mkrescue auto-detects the platforms
			name:    "legacy amd64",
			arch:    "amd64",
			version: "v1.9.0",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			options := prepareGRUBOptions(t, test.arch, test.version)

			generator, err := options.CreateGRUB(func(string, ...any) {})
			require.NoError(t, err)

			executor, ok := generator.(*iso.ExecutorOptions)
			require.True(t, ok)

			directoryArgs := xslices.Filter(executor.Arguments, func(arg string) bool {
				return strings.HasPrefix(arg, "--directory=")
			})

			if test.expectedDirectory == "" {
				assert.Empty(t, directoryArgs)
			} else {
				assert.Equal(t, []string{test.expectedDirectory}, directoryArgs)
			}
		})
	}
}

func prepareGRUBOptions(t *testing.T, arch, version string) iso.Options {
	t.Helper()

	tempDir := t.TempDir()

	kernelPath := filepath.Join(tempDir, "vmlinuz")
	require.NoError(t, os.WriteFile(kernelPath, []byte("kernel"), 0o644))

	initramfsPath := filepath.Join(tempDir, "initramfs.xz")
	require.NoError(t, os.WriteFile(initramfsPath, []byte("initramfs"), 0o644))

	return iso.Options{
		KernelPath:    kernelPath,
		InitramfsPath: initramfsPath,
		Cmdline:       "talos.platform=metal",

		Arch:    arch,
		Version: version,

		ScratchDir: filepath.Join(tempDir, "scratch"),
		OutPath:    filepath.Join(tempDir, "out.iso"),
	}
}
