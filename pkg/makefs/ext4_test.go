// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs_test

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/makefs"
)

// TestExt4Reproducibility tests that the ext4 filesystem is reproducible.
func TestExt4Reproducibility(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
	t.Setenv("SOURCE_DATE_EPOCH", "1234567890")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "reproducible-ext4.img")

	f, err := os.Create(tempFile)
	require.NoError(t, err)

	require.NoError(t, f.Truncate(512*1024*1024))
	require.NoError(t, f.Close())

	sourceDirectory := filepath.Join(tmpDir, "source")

	populateTestDir(t, sourceDirectory, []string{
		"file1.txt",
		"dir1/",
		"dir1/file2.txt",
		"dir1/subdir1/",
		"dir1/subdir1/file3.txt",
	})

	require.NoError(t, makefs.Ext4(t.Context(),
		tempFile,
		makefs.WithReproducible(true),
		makefs.WithLabel("TESTLABEL"),
		makefs.WithSourceDirectory(sourceDirectory),
	))

	fileData, err := os.Open(tempFile)
	require.NoError(t, err)

	sum1 := sha256.New()

	_, err = io.Copy(sum1, fileData)
	require.NoError(t, err)

	require.NoError(t, fileData.Close())

	// create the filesystem again
	require.NoError(t, makefs.Ext4(t.Context(),
		tempFile,
		makefs.WithReproducible(true),
		makefs.WithLabel("TESTLABEL"),
		makefs.WithSourceDirectory(sourceDirectory),
		makefs.WithForce(true)))

	fileData, err = os.Open(tempFile)
	require.NoError(t, err)

	sum2 := sha256.New()

	_, err = io.Copy(sum2, fileData)
	require.NoError(t, err)

	require.NoError(t, fileData.Close())

	assert.Equal(t, sum1.Sum(nil), sum2.Sum(nil), "ext4 filesystem is not reproducible")
}

// TestExt4Resize tests that the ext4 filesystem can be resized.
func TestExt4Resize(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "resize-ext4.img")

	if _, err := os.Create(tempFile); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := os.Truncate(tempFile, 64*1024*1024); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := makefs.Ext4(t.Context(), tempFile); err != nil {
		t.Fatalf("failed to create ext4 filesystem: %v", err)
	}

	if err := os.Truncate(tempFile, 128*1024*1024); err != nil {
		t.Fatalf("failed to resize file: %v", err)
	}

	assert.NoError(t, makefs.Ext4Resize(tempFile))
}
