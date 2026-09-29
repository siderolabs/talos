// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache_test

import (
	"archive/tar"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/cache"
)

func TestGenerateReusesUncompressedLayer(t *testing.T) {
	var layerData bytes.Buffer

	tarWriter := tar.NewWriter(&layerData)
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{
		Name: "file",
		Mode: 0o644,
		Size: 4,
	}))
	_, err := tarWriter.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())

	layer := static.NewLayer(layerData.Bytes(), types.OCILayer)
	image, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	digest, err := layer.Digest()
	require.NoError(t, err)

	var layerPulls atomic.Int32

	registryHandler := registry.New()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/blobs/"+digest.String()) {
			layerPulls.Add(1)
		}

		registryHandler.ServeHTTP(writer, request)
	}))
	t.Cleanup(server.Close)

	ref, err := name.NewTag(strings.TrimPrefix(server.URL, "http://")+"/test/image:latest", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, image))

	tempDir := t.TempDir()
	layerCachePath := filepath.Join(tempDir, "layers")
	firstCachePath := filepath.Join(tempDir, "first")
	secondCachePath := filepath.Join(tempDir, "second")

	for _, destination := range []string{firstCachePath, secondCachePath} {
		require.NoError(t, cache.Generate(
			[]string{ref.String()},
			[]string{"linux/amd64"},
			true,
			layerCachePath,
			destination,
			true,
			false,
		))
	}

	require.Equal(t, int32(1), layerPulls.Load())

	blobName := strings.ReplaceAll(digest.String(), ":", "-")

	for _, destination := range []string{firstCachePath, secondCachePath} {
		actual, err := os.ReadFile(filepath.Join(destination, "blob", blobName))
		require.NoError(t, err)
		require.True(t, bytes.Equal(layerData.Bytes(), actual), "cached layer bytes changed")
	}
}
