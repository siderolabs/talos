// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"archive/tar"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services/registry"
	"github.com/siderolabs/talos/pkg/imager/cache"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	cacheDir := t.TempDir()

	images := []string{
		fmt.Sprintf("%s:%s", constants.CoreDNSImage, constants.DefaultCoreDNSVersion),
		fmt.Sprintf("%s:%s", strings.ReplaceAll(constants.CoreDNSImage, "registry.k8s.io", "registry.k8s.io:443"), constants.DefaultCoreDNSVersion),
	}

	platform, err := v1.ParsePlatform("linux/amd64")
	assert.NoError(t, err)

	assert.NoError(t, cache.Generate(images, platform.String(), false, "", cacheDir))

	l, err := layout.ImageIndexFromPath(cacheDir)
	assert.NoError(t, err)

	m, err := l.IndexManifest()
	assert.NoError(t, err)

	image, err := l.Image(m.Manifests[0].Digest)
	assert.NoError(t, err)

	registryRoot := t.TempDir()

	tarExtract(t, image, registryRoot)

	logger := zaptest.NewLogger(t)

	it := func(yield func(string) bool) {
		if !yield(registryRoot) {
			return
		}
	}

	svc := registry.NewService(registry.NewMultiPathFS(it), logger)

	var wg sync.WaitGroup

	wg.Add(1)

	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	go func() {
		defer wg.Done()

		cancel(cmp.Or(svc.Run(ctx), errors.New("service exited")))
	}()

	defer wg.Wait()

	for _, image := range images {
		t.Run(image, func(t *testing.T) {
			ref, err := name.ParseReference(image)
			assert.NoError(t, err)

			manifest, err := crane.Manifest(ref.String())
			assert.NoError(t, err)

			rmt, err := remote.Get(ref, remote.WithPlatform(*platform))
			assert.NoError(t, err)

			for _, path := range []string{
				fmt.Sprintf("/v2/%s/manifests/%s?ns=%s", ref.Context().RepositoryStr(), constants.DefaultCoreDNSVersion, ref.Context().RegistryStr()),
				fmt.Sprintf("/v2/%s/manifests/%s?ns=%s", ref.Context().RepositoryStr(), rmt.Digest.String(), ref.Context().RegistryStr()),
			} {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+constants.RegistrydListenAddress+path, nil)
				assert.NoError(t, err)

				resp, err := http.DefaultClient.Do(req)
				assert.NoError(t, err)

				if resp != nil {
					t.Cleanup(func() {
						assert.NoError(t, resp.Body.Close())
					})
				}

				assert.Equal(t, http.StatusOK, pointer.SafeDeref(resp).StatusCode, "unexpected status code")
				assert.Equal(t, string(manifest), readAll(t, resp))
			}

			img, err := rmt.Image()
			assert.NoError(t, err)

			layers, err := img.Layers()
			assert.NoError(t, err)

			handleLayers(ctx, t, layers, ref)
		})
	}

	tests := []struct {
		name         string
		path         string
		method       string
		check        check.Check
		expectedCode int
	}{
		{
			name:         "HEAD /v2/",
			path:         "/v2/",
			method:       http.MethodHead,
			check:        check.NoError(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "GET /v2/",
			path:         "/v2/",
			method:       http.MethodGet,
			check:        check.NoError(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "HEAD /healthz",
			path:         "/healthz",
			method:       http.MethodHead,
			check:        check.NoError(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "GET /healthz",
			path:         "/healthz",
			method:       http.MethodGet,
			check:        check.NoError(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "GET /v2/alpine/manifests/3.20.3",
			path:         "/v2/alpine/manifests/3.20.3",
			method:       http.MethodGet,
			check:        check.NoError(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "GET /v2/alpine/manifests/3.20.3?ns=docker.io",
			path:         "/v2/alpine/manifests/3.20.3?ns=docker.io",
			method:       http.MethodGet,
			check:        check.NoError(),
			expectedCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, test.method, "http://"+constants.RegistrydListenAddress+test.path, nil)
			assert.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			test.check(t, err)

			if resp != nil {
				t.Cleanup(func() {
					assert.NoError(t, resp.Body.Close())
				})
			}

			assert.Equal(t, test.expectedCode, pointer.SafeDeref(resp).StatusCode, "unexpected status code, body is %s", readAll(t, resp))
		})
	}

	cancel(nil)
	wg.Wait()

	err = ctx.Err()
	if err == context.Canceled || err == context.DeadlineExceeded {
		err = nil
	}

	assert.NoError(t, err)
}

func handleLayers(ctx context.Context, t *testing.T, layers []v1.Layer, ref name.Reference) {
	for _, layer := range layers {
		dig, err := layer.Digest()
		assert.NoError(t, err)

		path := fmt.Sprintf("/v2/%s/blobs/%s?ns=%s", ref.Context().RepositoryStr(), dig, ref.Context().RegistryStr())

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, "http://"+constants.RegistrydListenAddress+path, nil)
		assert.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)

		if resp != nil {
			t.Cleanup(func() {
				assert.NoError(t, resp.Body.Close())
			})
		}

		assert.Equal(t, http.StatusOK, pointer.SafeDeref(resp).StatusCode, "unexpected status code")
	}
}

func tarExtract(t *testing.T, img v1.Image, dest string) {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		pipeWriter.CloseWithError(crane.Export(img, pipeWriter))
	}()

	tr := tar.NewReader(pipeReader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		assert.NoError(t, err)

		switch header.Typeflag {
		case tar.TypeDir:
			assert.NoError(t, os.MkdirAll(filepath.Join(dest, header.Name), 0o755))
		case tar.TypeReg:
			f, err := os.Create(filepath.Join(dest, header.Name))
			assert.NoError(t, err)

			_, err = io.Copy(f, tr)
			assert.NoError(t, err)
		default:
			assert.Failf(t, "unexpected tar entry type", "type: %v", header.Typeflag)
		}
	}
}

func readAll(t *testing.T, resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return "<no response>"
	}

	var builder strings.Builder

	_, err := io.Copy(&builder, resp.Body)
	assert.NoError(t, err)

	if builder.String() == "" {
		return "<empty response>"
	}

	return builder.String()
}
