// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
)

// tarGz builds a .tar.gz from the given headers, with the body of each regular
// file entry taken from bodies by name.
func tarGz(t *testing.T, headers []*tar.Header, bodies map[string]string) io.ReadCloser {
	t.Helper()

	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)

	for _, hdr := range headers {
		body := bodies[hdr.Name]
		hdr.Size = int64(len(body))

		require.NoError(t, tw.WriteHeader(hdr))

		if body != "" {
			_, err := tw.Write([]byte(body))
			require.NoError(t, err)
		}
	}

	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())

	return io.NopCloser(&buf)
}

// TestExtractTarGzSymlinkEscape verifies symlink escape on extraction.
func TestExtractTarGzSymlinkEscape(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		linkname func(outside string) string
	}{
		{
			name:     "absolute link target",
			linkname: func(outside string) string { return outside },
		},
		{
			name:     "relative link target",
			linkname: func(outside string) string { return "../outside" },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			root := filepath.Join(base, "root")
			outside := filepath.Join(base, "outside")

			require.NoError(t, os.Mkdir(root, 0o755))
			require.NoError(t, os.Mkdir(outside, 0o755))

			headers := []*tar.Header{
				{
					Name:     "esc",
					Linkname: test.linkname(outside),
					Typeflag: tar.TypeSymlink,
					Mode:     0o777,
				},
				{
					Name:     "esc/PWNED",
					Typeflag: tar.TypeReg,
					Mode:     0o4755,
				},
			}

			archive := tarGz(t, headers, map[string]string{"esc/PWNED": "payload"})

			err := helpers.ExtractTarGz(root, archive)

			escaped := filepath.Join(outside, "PWNED")

			_, statErr := os.Lstat(escaped)
			assert.True(t, os.IsNotExist(statErr), "a tar entry wrote outside the extraction root: %s (extract error: %v)", escaped, err)

			// and it fails loudly: reporting success while skipping the entry would
			// leave the operator believing the copy completed.
			assert.Error(t, err)
		})
	}
}

// TestExtractTarGzDropsSetuid verifies that setuid and setgid bits are dropped on extraction.
func TestExtractTarGzSetuid(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	headers := []*tar.Header{{Name: "suid", Typeflag: tar.TypeReg, Mode: 0o4755}}

	archive := tarGz(t, headers, map[string]string{"suid": "payload"})

	require.NoError(t, helpers.ExtractTarGz(root, archive))

	info, err := os.Stat(filepath.Join(root, "suid"))
	require.NoError(t, err)

	assert.Zero(t, info.Mode()&(os.ModeSetuid|os.ModeSetgid), "extracted file kept mode %s", info.Mode())
}

// TestExtractTarGzNormal: a well-formed archive still extracts.
func TestExtractTarGzNormal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	headers := []*tar.Header{
		{Name: "dir", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "dir/file", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "dir/link", Linkname: "file", Typeflag: tar.TypeSymlink, Mode: 0o777},
	}

	archive := tarGz(t, headers, map[string]string{"dir/file": "hello"})

	require.NoError(t, helpers.ExtractTarGz(root, archive))

	body, err := os.ReadFile(filepath.Join(root, "dir", "file"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(body))

	target, err := os.Readlink(filepath.Join(root, "dir", "link"))
	require.NoError(t, err)
	assert.Equal(t, "file", target)

	body, err = os.ReadFile(filepath.Join(root, "dir", "link"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(body))
}
