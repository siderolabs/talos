// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package filemap_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/filemap"
)

func TestFileMap(t *testing.T) {
	tempDir := t.TempDir()

	assert.NoError(t, os.MkdirAll(filepath.Join(tempDir, "foo/a/b"), 0o755))
	assert.NoError(t, os.MkdirAll(filepath.Join(tempDir, "foo/c"), 0o755))
	assert.NoError(t, os.MkdirAll(filepath.Join(tempDir, "foo/d"), 0o750))

	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "foo/a/b/normal"), nil, 0o644))
	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "foo/c/executable"), []byte("world"), 0o755))

	artifacts, err := filemap.Walk(tempDir, "")
	assert.NoError(t, err)

	assert.Equal(
		t,
		[]filemap.File{
			{
				ImagePath:  "foo",
				SourcePath: filepath.Join(tempDir, "foo"),
				ImageMode:  0o755,
			},
			{
				ImagePath:  "foo/a",
				SourcePath: filepath.Join(tempDir, "foo/a"),
				ImageMode:  0o755,
			},
			{
				ImagePath:  "foo/a/b",
				SourcePath: filepath.Join(tempDir, "foo/a/b"),
				ImageMode:  0o755,
			},
			{
				ImagePath:  "foo/a/b/normal",
				SourcePath: filepath.Join(tempDir, "foo/a/b/normal"),
				ImageMode:  0o644,
			},
			{
				ImagePath:  "foo/c",
				SourcePath: filepath.Join(tempDir, "foo/c"),
				ImageMode:  0o755,
			},
			{
				ImagePath:  "foo/c/executable",
				SourcePath: filepath.Join(tempDir, "foo/c/executable"),
				ImageMode:  0o755,
			},
			{
				ImagePath:  "foo/d",
				SourcePath: filepath.Join(tempDir, "foo/d"),
				ImageMode:  0o750,
			},
		},
		artifacts,
	)
}

// TestLayer exercises the digest, diffID and compressed-read paths the way go-containerregistry
// does when building and writing an image. Run with -race, it guards against re-introducing
// repeated compression of the layer (each compression spins up a streaming gzip goroutine, and
// the GC-reused flate buffers across those goroutines trip the race detector).
func TestLayer(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "a/b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a/b/file"), []byte("hello world"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a/executable"), []byte("payload"), 0o755))

	artifacts, err := filemap.Walk(tempDir, "")
	require.NoError(t, err)

	layer, err := filemap.Layer(artifacts)
	require.NoError(t, err)

	_, err = layer.Digest()
	require.NoError(t, err)

	_, err = layer.DiffID()
	require.NoError(t, err)

	// read the compressed stream multiple times, mirroring digest + tarball write
	for range 3 {
		rc, err := layer.Compressed()
		require.NoError(t, err)

		_, err = io.Copy(io.Discard, rc)
		require.NoError(t, err)

		require.NoError(t, rc.Close())
	}
}
