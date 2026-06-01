// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs_test

import (
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/makefs"
)

func TestGenerateProtofile(t *testing.T) {
	tests := []struct {
		name      string
		protoFile string
		setupFunc func(t *testing.T, dir string)
	}{
		{
			name:      "simple directory with files",
			protoFile: "testdata/simple_directory.proto",
			setupFunc: func(t *testing.T, dir string) {
				populateTestDir(t, dir, []string{
					"file1.txt",
					"file2.txt",
				})
			},
		},
		{
			name:      "nested directories",
			protoFile: "testdata/nested_directories.proto",
			setupFunc: func(t *testing.T, dir string) {
				populateTestDir(t, dir, []string{
					"subdir/",
					"root.txt",
					"subdir/nested.txt",
				})
			},
		},

		{
			name:      "symlinks",
			protoFile: "testdata/symlinks.proto",
			setupFunc: func(t *testing.T, dir string) {
				populateTestDir(t, dir, []string{
					"target.txt",
				})

				link := filepath.Join(dir, "link.txt")
				require.NoError(t, os.Symlink("target.txt", link))
			},
		},
		{
			name:      "deeply nested directories",
			protoFile: "testdata/deeply_nested.proto",
			setupFunc: func(t *testing.T, dir string) {
				populateTestDir(t, dir, []string{
					"root1.txt",
					"root2.txt",
					"subdir/",
					"subdir/file1.txt",
					"subdir/file2.txt",
					"subdir/nested/",
					"subdir/nested/deep.txt",
					"anotherdir/",
					"anotherdir/file.txt",
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testDir := filepath.Join(tmpDir, "testdata")
			require.NoError(t, os.Mkdir(testDir, 0o755))

			tt.setupFunc(t, testDir)

			r, err := makefs.GenerateProtofile(testDir)
			require.NoError(t, err)

			protoFileData, err := io.ReadAll(r)
			require.NoError(t, err)

			// Read expected protofile
			protoData, err := os.ReadFile(tt.protoFile)
			require.NoError(t, err, "failed to read protofile %s", tt.protoFile)

			protoDataStr := strings.ReplaceAll(string(protoData), "FILEPATH", testDir)

			assert.Equal(t, protoDataStr, string(protoFileData), "protofile output does not match expected %s", tt.protoFile)
		})
	}
}

func TestProtofileRejectsSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdata")
	require.NoError(t, os.Mkdir(testDir, 0o755))

	// Create a file with space in name
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "file with space.txt"), []byte("content"), 0o644))

	_, err := makefs.GenerateProtofile(testDir)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "spaces not allowed")
}

func TestProtofileErrorCases(t *testing.T) {
	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := makefs.GenerateProtofile("/nonexistent/path")
		require.Error(t, err)
	})

	t.Run("not a directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

		_, err := makefs.GenerateProtofile(filePath)
		require.Error(t, err)

		assert.Contains(t, err.Error(), "not a directory")
	})
}
