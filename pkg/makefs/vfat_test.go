// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/siderolabs/go-cmd/pkg/cmd"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/makefs"
)

func TestVFATWithSourceDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory structure
	sourceDir := filepath.Join(tempDir, "source")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "subdir"), 0o755))

	// Create test files
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "file1.txt"), []byte("content1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "file2.txt"), []byte("content2"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "subdir", "file3.txt"), []byte("content3"), 0o644))

	// Create VFAT image
	vfatImg := filepath.Join(tempDir, "test.img")

	// Create a 10MB image file
	f, err := os.Create(vfatImg)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(10*1024*1024))
	require.NoError(t, f.Close())

	// Format and populate VFAT
	err = makefs.VFAT(vfatImg, makefs.WithLabel("TEST"), makefs.WithSourceDirectory(sourceDir))
	require.NoError(t, err)

	// Verify file contents using mcopy
	extractDir := filepath.Join(tempDir, "extract")
	require.NoError(t, os.MkdirAll(extractDir, 0o755))

	// Extract all files
	_, err = cmd.Run("mcopy", "-s", "-i", vfatImg, "::/", extractDir)
	require.NoError(t, err)

	// Verify extracted files with full filenames
	content, err := os.ReadFile(filepath.Join(extractDir, "file1.txt"))
	require.NoError(t, err)
	require.Equal(t, "content1", string(content))

	content, err = os.ReadFile(filepath.Join(extractDir, "file2.txt"))
	require.NoError(t, err)
	require.Equal(t, "content2", string(content))

	content, err = os.ReadFile(filepath.Join(extractDir, "subdir", "file3.txt"))
	require.NoError(t, err)
	require.Equal(t, "content3", string(content))
}
