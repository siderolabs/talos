// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package filemap_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

	layer, err := filemap.Layer(t.TempDir(), artifacts)
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

// TestLayerDigestParity pins the staged layer against the one go-containerregistry builds from an
// uncompressed opener.
//
// Layer compresses ahead of time at a level picked to match go-containerregistry's private default,
// and a dependency bump which changed that default would otherwise silently change the digest
// of every artifacts layer we publish.
func TestLayerDigestParity(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "a/b"), 0o755))
	// compressible, so that the compression level is what decides the digest.
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a/b/file"), bytes.Repeat([]byte("hello world"), 4096), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a/executable"), []byte("payload"), 0o755))

	artifacts, err := filemap.Walk(tempDir, "")
	require.NoError(t, err)

	// sorts artifacts in place, so the reference layer below sees the same entry order.
	staged, err := filemap.Layer(t.TempDir(), artifacts)
	require.NoError(t, err)

	reference, err := tarball.LayerFromOpener(
		func() (io.ReadCloser, error) { return filemap.Build(artifacts), nil },
		tarball.WithMediaType(types.OCILayer),
	)
	require.NoError(t, err)

	stagedDigest, err := staged.Digest()
	require.NoError(t, err)

	referenceDigest, err := reference.Digest()
	require.NoError(t, err)

	assert.Equal(t, referenceDigest, stagedDigest)

	stagedSize, err := staged.Size()
	require.NoError(t, err)

	referenceSize, err := reference.Size()
	require.NoError(t, err)

	assert.Equal(t, referenceSize, stagedSize)
}

// TestLayerHeapUsage guards against the layer memoizing its compressed contents on the heap,
// the way tarball.WithCompressedCaching does.
func TestLayerHeapUsage(t *testing.T) {
	const payloadSize = 32 << 20

	sourceDir := t.TempDir()

	// random, hence incompressible: the layer cannot end up much smaller than the payload.
	payload := make([]byte, payloadSize)
	_, err := rand.Read(payload)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "payload"), payload, 0o644))

	artifacts, err := filemap.Walk(sourceDir, "")
	require.NoError(t, err)

	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	layer, err := filemap.Layer(t.TempDir(), artifacts)
	require.NoError(t, err)

	size, err := layer.Size()
	require.NoError(t, err)
	require.Greater(t, size, int64(payloadSize/2), "payload compressed further than expected, test is not measuring anything")

	runtime.GC()
	runtime.ReadMemStats(&after)

	runtime.KeepAlive(layer)

	assert.Less(t, after.HeapAlloc, before.HeapAlloc+payloadSize/2, "layer retains its compressed contents on the heap")
}
