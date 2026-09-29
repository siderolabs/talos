// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !race

package filemap_test

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/filemap"
)

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
