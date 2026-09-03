// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// fakeGrubMkimage puts a stub 'grub-mkimage' into PATH which simply creates an empty output file.
func fakeGrubMkimage(t *testing.T) {
	t.Helper()

	binDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(binDir, "grub-mkimage"), []byte(`#!/bin/sh
while [ "$#" -gt 0 ]; do
	if [ "$1" = "--output" ]; then
		shift
		echo "fake-grub-efi" > "$1"

		exit 0
	fi

	shift
done

exit 1
`), 0o755))

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestPrepareBootPartitionsImageMode verifies the layout of the staged boot assets in image mode:
// the EFI assets (including the ones written by the overlay installer) should end up in the EFI
// partition source directory, and not in the BOOT partition source directory.
func TestPrepareBootPartitionsImageMode(t *testing.T) {
	fakeGrubMkimage(t)

	mountPrefix := t.TempDir()
	assetsDir := t.TempDir()

	kernelPath := filepath.Join(assetsDir, "vmlinuz")
	initramfsPath := filepath.Join(assetsDir, "initramfs.xz")

	require.NoError(t, os.WriteFile(kernelPath, []byte("fake-kernel"), 0o644))
	require.NoError(t, os.WriteFile(initramfsPath, []byte("fake-initramfs"), 0o644))

	// emulate an SBC overlay installer: it writes the assets it needs on the EFI partition
	// to <mount prefix>/boot/EFI
	extraInstallStep := func() error {
		if err := os.MkdirAll(filepath.Join(mountPrefix, constants.EFIMountPoint), 0o755); err != nil {
			return err
		}

		return os.WriteFile(filepath.Join(mountPrefix, constants.EFIMountPoint, "start4.elf"), []byte("fake-firmware"), 0o644)
	}

	partitionOptions, err := grub.NewConfig().PrepareBootPartitions(options.InstallOptions{
		Arch:        "arm64",
		Version:     "v1.14.0",
		ImageMode:   true,
		MountPrefix: mountPrefix,
		BootAssets: options.BootAssets{
			KernelPath:    kernelPath,
			InitramfsPath: initramfsPath,
		},
		ExtraInstallStep: extraInstallStep,
		Printf:           t.Logf,
	})
	require.NoError(t, err)

	sourceDirectories := map[string]string{}

	for _, partitionOption := range partitionOptions {
		sourceDirectories[partitionOption.Label] = partitionOption.SourceDirectory
	}

	assert.Equal(t, map[string]string{
		constants.EFIPartitionLabel:      filepath.Join(mountPrefix, "EFI"),
		constants.BIOSGrubPartitionLabel: "",
		constants.BootPartitionLabel:     filepath.Join(mountPrefix, constants.BootMountPoint),
	}, sourceDirectories)

	efiSourceDirectory := sourceDirectories[constants.EFIPartitionLabel]
	bootSourceDirectory := sourceDirectories[constants.BootPartitionLabel]

	// GRUB EFI binary and the overlay assets should be on the EFI partition
	assert.FileExists(t, filepath.Join(efiSourceDirectory, "EFI", "boot", "BOOTAA64.efi"))
	assert.FileExists(t, filepath.Join(efiSourceDirectory, "start4.elf"))

	// kernel, initramfs and grub.cfg should be on the BOOT partition
	assert.FileExists(t, filepath.Join(bootSourceDirectory, "A", constants.KernelAsset))
	assert.FileExists(t, filepath.Join(bootSourceDirectory, "A", constants.InitramfsAsset))
	assert.FileExists(t, filepath.Join(mountPrefix, grub.ConfigPath))

	// nothing EFI-related should leak into the BOOT partition
	assert.NoDirExists(t, filepath.Join(bootSourceDirectory, "EFI"))
}

// TestPrepareBootPartitionsInstallMode verifies that in install mode no assets are staged, as they
// are copied directly to the mounted partitions in the Install step.
func TestPrepareBootPartitionsInstallMode(t *testing.T) {
	t.Parallel()

	mountPrefix := t.TempDir()

	partitionOptions, err := grub.NewConfig().PrepareBootPartitions(options.InstallOptions{
		Arch:        "arm64",
		Version:     "v1.14.0",
		MountPrefix: mountPrefix,
		Printf:      t.Logf,
	})
	require.NoError(t, err)

	for _, partitionOption := range partitionOptions {
		assert.Empty(t, partitionOption.SourceDirectory, "partition %q", partitionOption.Label)
	}

	assert.Empty(t, dirEntries(t, mountPrefix))
}

func dirEntries(t *testing.T, path string) []string {
	t.Helper()

	entries, err := os.ReadDir(path)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}
