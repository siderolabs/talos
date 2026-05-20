// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/makefs"
)

func TestBTRFSInfo(t *testing.T) { //nolint:tparallel
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	for _, test := range []struct {
		name string

		size int64
	}{
		{
			name: "256M",

			size: 256 * 1024 * 1024,
		},
		{
			name: "1G",

			size: 1024 * 1024 * 1024,
		},
		{
			name: "4G",

			size: 4 * 1024 * 1024 * 1024,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()

			tempFile := filepath.Join(tmpDir, "btrfs.img")

			require.NoError(t, os.WriteFile(tempFile, nil, 0o644))
			require.NoError(t, os.Truncate(tempFile, test.size))

			require.NoError(t, makefs.BTRFS(t.Context(), tempFile, makefs.WithLabel("TESTLABEL")))

			var stdout bytes.Buffer

			cmd := exec.CommandContext(t.Context(), "btrfs", "inspect-internal", "dump-super", tempFile)
			cmd.Stdout = &stdout
			require.NoError(t, cmd.Run())

			out := stdout.String()

			assert.Regexp(t, `(?m)^label\s+TESTLABEL$`, out)
			assert.Regexp(t, `(?m)^total_bytes\s+`+strconv.FormatInt(test.size, 10)+`$`, out)
			assert.Regexp(t, `(?m)^nodesize\s+16384$`, out)
		})
	}
}

func TestBTRFSCustomSectorSize(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "btrfs.img")

	require.NoError(t, os.WriteFile(tempFile, nil, 0o644))
	require.NoError(t, os.Truncate(tempFile, 256*1024*1024))

	// 16384 differs from the default (page size, 4096 on x86_64) on every
	// common host arch, so a missing flag would be visible in the dump.
	require.NoError(t, makefs.BTRFS(t.Context(), tempFile, makefs.WithSectorSize(16384)))

	var stdout bytes.Buffer

	cmd := exec.CommandContext(t.Context(), "btrfs", "inspect-internal", "dump-super", tempFile)
	cmd.Stdout = &stdout
	require.NoError(t, cmd.Run())

	assert.Regexp(t, `(?m)^sectorsize\s+16384$`, stdout.String())
}

func TestBTRFSReproducibleUUID(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "btrfs.img")

	require.NoError(t, os.WriteFile(tempFile, nil, 0o644))
	require.NoError(t, os.Truncate(tempFile, 256*1024*1024))

	require.NoError(t, makefs.BTRFS(
		t.Context(),
		tempFile,
		makefs.WithReproducible(true),
		makefs.WithLabel("TESTLABEL"),
	))

	expectedUUID := makefs.GUIDFromLabel("TESTLABEL").String()

	var stdout bytes.Buffer

	cmd := exec.CommandContext(t.Context(), "btrfs", "inspect-internal", "dump-super", tempFile)
	cmd.Stdout = &stdout
	require.NoError(t, cmd.Run())

	out := stdout.String()

	assert.Regexp(t, `(?m)^fsid\s+`+expectedUUID+`$`, out)
	assert.Regexp(t, `(?m)^dev_item.uuid\s+`+expectedUUID+`$`, out)
}

func TestBTRFSWithSourceDirectory(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "btrfs.img")

	require.NoError(t, os.WriteFile(tempFile, nil, 0o644))
	require.NoError(t, os.Truncate(tempFile, 256*1024*1024))

	sourceDirectory := filepath.Join(tmpDir, "source")

	populateTestDir(t, sourceDirectory, []string{
		"file1.txt",
		"dir1/",
		"dir1/file2.txt",
		"dir1/subdir1/",
		"dir1/subdir1/file3.txt",
	})

	require.NoError(t, makefs.BTRFS(
		t.Context(),
		tempFile,
		makefs.WithLabel("WITHSRC"),
		makefs.WithSourceDirectory(sourceDirectory),
	))

	// btrfs check walks the filesystem; if mkfs.btrfs -r left the FS in a
	// sane state, this will succeed.
	cmd := exec.CommandContext(t.Context(), "btrfs", "check", tempFile)
	require.NoError(t, cmd.Run())
}

func TestBTRFSRepair(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "btrfs.img")

	require.NoError(t, os.WriteFile(tempFile, nil, 0o644))
	require.NoError(t, os.Truncate(tempFile, 256*1024*1024))

	require.NoError(t, makefs.BTRFS(t.Context(), tempFile, makefs.WithLabel("REPAIR")))

	// running --repair on a clean filesystem is a no-op but exercises the
	// command-line wrapper end to end.
	assert.NoError(t, makefs.BTRFSRepair(t.Context(), tempFile))
}

func TestBTRFSForce(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "btrfs.img")

	require.NoError(t, os.WriteFile(tempFile, nil, 0o644))
	require.NoError(t, os.Truncate(tempFile, 256*1024*1024))

	require.NoError(t, makefs.BTRFS(t.Context(), tempFile, makefs.WithLabel("FIRST")))

	// without -f, mkfs.btrfs refuses to overwrite an existing filesystem
	require.Error(t, makefs.BTRFS(t.Context(), tempFile, makefs.WithLabel("SECOND")))

	require.NoError(t, makefs.BTRFS(t.Context(), tempFile, makefs.WithLabel("SECOND"), makefs.WithForce(true)))

	var stdout bytes.Buffer

	cmd := exec.CommandContext(t.Context(), "btrfs", "inspect-internal", "dump-super", tempFile)
	cmd.Stdout = &stdout
	require.NoError(t, cmd.Run())

	assert.Regexp(t, `(?m)^label\s+SECOND$`, stdout.String())
}
