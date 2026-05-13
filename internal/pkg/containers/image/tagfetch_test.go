// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/containerd/errdefs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// TestTagFetcher covers the by-tag manifest fallback against an httptest server,
// so the regression for siderolabs/talos#13342 is exercised without depending on
// live CDN behavior.
func TestTagFetcher(t *testing.T) {
	t.Parallel()

	const (
		repoStr = "library/etcd"
		tag     = "v3.6.11"
	)

	// NewTagFetcher only returns the body bytes and validates against expectedDigest,
	// so the body just has to be a deterministic byte sequence — the JSON shape is for
	// readability.
	manifestBody := []byte(`{"schemaVersion":2,"layers":[]}`)
	expectedDigest := digest.FromBytes(manifestBody)

	tagPath := "/v2/" + repoStr + "/manifests/" + tag
	digestPath := "/v2/" + repoStr + "/manifests/" + expectedDigest.String()

	for _, tt := range []struct {
		name string

		handler http.HandlerFunc

		expectedBody   []byte
		expectedErrIs  error
		expectedErrSub string
	}{
		{
			name: "by-digest 404, by-tag 200 returns body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case digestPath:
					w.WriteHeader(http.StatusNotFound)
				case tagPath:
					w.Header().Set("Docker-Content-Digest", expectedDigest.String())
					w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
					w.Write(manifestBody) //nolint:errcheck
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			},
			expectedBody: manifestBody,
		},
		{
			name: "by-tag 200 without Docker-Content-Digest header still verifies body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tagPath {
					w.WriteHeader(http.StatusNotFound)

					return
				}

				w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				w.Write(manifestBody) //nolint:errcheck
			},
			expectedBody: manifestBody,
		},
		{
			name: "by-tag 404 surfaces NotFound",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedErrIs: errdefs.ErrNotFound,
		},
		{
			name: "by-tag body digest mismatch is reported",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tagPath {
					w.WriteHeader(http.StatusNotFound)

					return
				}

				w.Write([]byte("not-the-real-manifest")) //nolint:errcheck
			},
			expectedErrSub: "tag fetch digest mismatch",
		},
		{
			name: "by-tag mismatched Docker-Content-Digest header is reported",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tagPath {
					w.WriteHeader(http.StatusNotFound)

					return
				}

				w.Header().Set("Docker-Content-Digest", "sha256:dead")
				w.Write(manifestBody) //nolint:errcheck
			},
			expectedErrSub: "tag fetch server digest mismatch",
		},
		{
			name: "by-tag 5xx surfaces a non-NotFound error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedErrSub: "unexpected status 500",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(tt.handler)
			t.Cleanup(srv.Close)

			// Route requests for the synthetic registry through the test server using
			// a mirror, the same way a user would set up an in-cluster mirror.
			cfg := &mockConfig{
				mirrors: map[string]*cri.RegistryMirrorConfig{
					"example.com": {
						MirrorEndpoints:    []cri.RegistryEndpointConfig{{EndpointEndpoint: srv.URL}},
						MirrorSkipFallback: true,
					},
				},
			}

			fetcher := image.NewTagFetcher(cfg)

			repo, err := name.NewRepository("example.com/" + repoStr)
			require.NoError(t, err)

			body, err := fetcher(t.Context(), repo, tag, expectedDigest)

			if tt.expectedErrIs != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErrIs), "expected error to wrap %v, got %v", tt.expectedErrIs, err)

				return
			}

			if tt.expectedErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrSub)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody, body)
		})
	}
}

// TestTagFetcherMaxBodySize verifies that the fetcher rejects oversize bodies
// (digest of the truncated read won't match the expected digest).
func TestTagFetcherMaxBodySize(t *testing.T) {
	t.Parallel()

	const (
		repoStr = "library/etcd"
		tag     = "v3.6.11"
	)

	// A body larger than maxTagManifestSize (4 MiB) gets truncated by the fetcher,
	// so its digest will not match what the server claims via Docker-Content-Digest.
	oversize := strings.Repeat("a", 5*1024*1024)
	advertisedDigest := digest.FromBytes([]byte(oversize))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Docker-Content-Digest", advertisedDigest.String())
		io.WriteString(w, oversize) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)

	cfg := &mockConfig{
		mirrors: map[string]*cri.RegistryMirrorConfig{
			"example.com": {
				MirrorEndpoints:    []cri.RegistryEndpointConfig{{EndpointEndpoint: srv.URL}},
				MirrorSkipFallback: true,
			},
		},
	}

	fetcher := image.NewTagFetcher(cfg)

	repo, err := name.NewRepository("example.com/" + repoStr)
	require.NoError(t, err)

	_, err = fetcher(t.Context(), repo, tag, advertisedDigest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag fetch digest mismatch")
}
