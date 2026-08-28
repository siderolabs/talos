// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package archiver_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/archiver"
)

func TestUntar(t *testing.T) {
	t.Parallel()

	type tarEntry struct {
		header  tar.Header
		payload []byte
	}

	for _, tc := range []struct {
		name string

		entries []tarEntry

		verification func(t *testing.T, dir string, xattrs map[string]string)
	}{
		{
			name: "nested paths",
			entries: []tarEntry{
				{
					header: tar.Header{
						Name: "nested/path/file.txt",
						Mode: 0o644,
					},
					payload: []byte("hello"),
				},
			},
			verification: func(t *testing.T, dir string, xattrs map[string]string) {
				data, err := os.ReadFile(filepath.Join(dir, "nested/path/file.txt"))
				require.NoError(t, err)

				assert.Equal(t, []byte("hello"), data)
				assert.Empty(t, xattrs)
			},
		},
		{
			name: "abs path",

			entries: []tarEntry{
				{
					header: tar.Header{
						Name: "/file.txt",
						Mode: 0o644,
					},
					payload: []byte("abs"),
				},
			},
			verification: func(t *testing.T, dir string, xattrs map[string]string) {
				data, err := os.ReadFile(filepath.Join(dir, "file.txt"))
				require.NoError(t, err)

				assert.Equal(t, []byte("abs"), data)
				assert.Empty(t, xattrs)
			},
		},
		{
			name: "xattrs",

			entries: []tarEntry{
				{
					header: tar.Header{
						Name:   "file.txt",
						Mode:   0o644,
						Xattrs: map[string]string{"security.selinux": "test_t"},
					},
					payload: []byte("xattrs"),
				},
			},
			verification: func(t *testing.T, dir string, xattrs map[string]string) {
				assert.Equal(
					t, map[string]string{
						filepath.Join(dir, "file.txt"): "test_t",
					},
					xattrs,
				)
			},
		},
		{
			name: "setuid file",

			entries: []tarEntry{
				{
					header: tar.Header{
						Name: "setuid-file",
						Mode: 0o4755,
					},
					payload: []byte("setuid"),
				},
			},
			verification: func(t *testing.T, dir string, xattrs map[string]string) {
				info, err := os.Stat(filepath.Join(dir, "setuid-file"))
				require.NoError(t, err)

				assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
				assert.NotZero(t, info.Mode()&os.ModeSetuid)
				assert.Empty(t, xattrs)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			tw := tar.NewWriter(&buf)

			for _, entry := range tc.entries {
				header := entry.header
				header.Size = int64(len(entry.payload))

				require.NoError(t, tw.WriteHeader(&header))
				_, err := tw.Write(entry.payload)
				require.NoError(t, err)
			}

			require.NoError(t, tw.Close())

			dir := t.TempDir()

			xattrsMap := map[string]string{}

			require.NoError(t, archiver.Untar(t.Context(), bytes.NewReader(buf.Bytes()), dir, xattrsMap))

			tc.verification(t, dir, xattrsMap)
		})
	}
}

func TestUntarUnsafeSymlink(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "symlink",
		Typeflag: tar.TypeSymlink,
		Linkname: "../passwd",
		Mode:     0o644,
	}))

	payload := []byte("oops!")

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "symlink/a.txt",
		Mode: 0o644,
		Size: int64(len(payload)),
	}))

	_, err := tw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	dir := t.TempDir()

	extractDir := filepath.Join(dir, "extract")
	symlinkedDir := filepath.Join(dir, "symlink")

	require.NoError(t, os.MkdirAll(extractDir, 0o755))
	require.NoError(t, os.MkdirAll(symlinkedDir, 0o755))

	err = archiver.Untar(t.Context(), bytes.NewReader(buf.Bytes()), extractDir, nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "path escapes from parent")
}
