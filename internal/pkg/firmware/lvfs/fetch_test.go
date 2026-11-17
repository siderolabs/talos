// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/jcat/jcattest"
	"github.com/siderolabs/talos/internal/pkg/firmware/lvfs"
)

// server serves the files under /downloads/, counting requests by path.
type server struct {
	*httptest.Server

	files map[string][]byte

	mu      sync.Mutex
	hits    map[string]int
	queries []string
}

func newServer(t *testing.T) *server {
	t.Helper()

	s := &server{files: map[string][]byte{}, hits: map[string]int{}}

	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits[r.URL.Path]++
		s.queries = append(s.queries, r.URL.RawQuery)
		s.mu.Unlock()

		data, ok := s.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)

			return
		}

		w.Write(data) //nolint:errcheck
	}))

	t.Cleanup(s.Close)

	return s
}

func (s *server) fetcher(pki *jcattest.PKI, name string) *lvfs.Fetcher {
	return &lvfs.Fetcher{
		Client:      s.Client(),
		MetadataURL: s.URL + "/downloads/" + name,
		Roots:       pki.Roots,
	}
}

// publish serves data as the named metadata with a signature over it.
func (s *server) publish(t *testing.T, pki *jcattest.PKI, name string, data []byte) {
	t.Helper()

	s.files["/downloads/"+name] = data
	s.files["/downloads/"+name+".jcat"] = jcattest.Build(t, pki.Item(t, "firmware-1-stable"+name[len("firmware"):], []string{name}, data))
}

func compressZstd(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	enc, err := zstd.NewWriter(&buf)
	require.NoError(t, err)

	_, err = enc.Write(data)
	require.NoError(t, err)
	require.NoError(t, enc.Close())

	return buf.Bytes()
}

func compressGzip(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)

	_, err := gz.Write(data)
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	return buf.Bytes()
}

func TestMetadata(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)
	xml := []byte(catalogXML)

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"firmware.xml.zst", compressZstd(t, xml)},
		{"firmware.xml.gz", compressGzip(t, xml)},
		{"firmware.xml", xml},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newServer(t)
			s.publish(t, pki, tc.name, tc.data)

			got, err := s.fetcher(pki, tc.name).Metadata(context.Background())
			require.NoError(t, err)
			assert.Equal(t, xml, got)

			assert.Equal(t, 1, s.hits["/downloads/"+tc.name])
			assert.Equal(t, 1, s.hits["/downloads/"+tc.name+".jcat"])
		})
	}
}

func TestMetadataURLWithQuery(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)
	xml := []byte(catalogXML)

	s := newServer(t)
	s.publish(t, pki, "firmware.xml.zst", compressZstd(t, xml))

	fetcher := s.fetcher(pki, "firmware.xml.zst?token=abc")

	// the signature is the metadata's path plus .jcat, with the same query
	got, err := fetcher.Metadata(context.Background())
	require.NoError(t, err)
	assert.Equal(t, xml, got)

	assert.Equal(t, []string{"token=abc", "token=abc"}, s.queries)
	assert.Equal(t, 1, s.hits["/downloads/firmware.xml.zst.jcat"])
}

func TestMetadataErrors(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)
	compressed := compressZstd(t, []byte(catalogXML))

	t.Run("missing metadata", func(t *testing.T) {
		t.Parallel()

		s := newServer(t)

		_, err := s.fetcher(pki, "firmware.xml.zst").Metadata(context.Background())
		require.ErrorContains(t, err, `error downloading metadata: unexpected status "404 Not Found"`)
	})

	t.Run("missing signature", func(t *testing.T) {
		t.Parallel()

		s := newServer(t)
		s.files["/downloads/firmware.xml.zst"] = compressed

		_, err := s.fetcher(pki, "firmware.xml.zst").Metadata(context.Background())
		require.ErrorContains(t, err, "error downloading metadata signature")
	})

	t.Run("tampered metadata", func(t *testing.T) {
		t.Parallel()

		s := newServer(t)
		s.publish(t, pki, "firmware.xml.zst", compressed)
		s.files["/downloads/firmware.xml.zst"] = compressZstd(t, []byte("<components/>"))

		_, err := s.fetcher(pki, "firmware.xml.zst").Metadata(context.Background())
		require.ErrorContains(t, err, "metadata signature: SHA-256 checksum mismatch")
	})

	t.Run("untrusted signer", func(t *testing.T) {
		t.Parallel()

		s := newServer(t)
		s.publish(t, pki, "firmware.xml.zst", compressed)

		_, err := s.fetcher(jcattest.NewPKI(t), "firmware.xml.zst").Metadata(context.Background())
		require.ErrorContains(t, err, "metadata signature: no valid PKCS7 signature")
	})

	t.Run("signature for another file", func(t *testing.T) {
		t.Parallel()

		s := newServer(t)
		s.publish(t, pki, "firmware.xml.zst", compressed)
		s.files["/downloads/firmware.xml.zst.jcat"] = jcattest.Build(t, pki.Item(t, "other.xml.zst", nil, compressed))

		_, err := s.fetcher(pki, "firmware.xml.zst").Metadata(context.Background())
		require.ErrorContains(t, err, `no jcat item for "firmware.xml.zst"`)
	})

	t.Run("corrupt compression", func(t *testing.T) {
		t.Parallel()

		s := newServer(t)
		s.publish(t, pki, "firmware.xml.zst", []byte("not zstd"))

		_, err := s.fetcher(pki, "firmware.xml.zst").Metadata(context.Background())
		require.ErrorContains(t, err, "error decompressing metadata")
	})

	t.Run("no file name", func(t *testing.T) {
		t.Parallel()

		s := newServer(t)

		_, err := (&lvfs.Fetcher{Client: s.Client(), MetadataURL: s.URL + "/"}).Metadata(context.Background())
		require.ErrorContains(t, err, "has no file name")
	})

	t.Run("canceled", func(t *testing.T) {
		t.Parallel()

		s := newServer(t)
		s.publish(t, pki, "firmware.xml.zst", compressed)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := s.fetcher(pki, "firmware.xml.zst").Metadata(ctx)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestRelease(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)
	container := []byte("MSCF container bytes")
	sum := sha256.Sum256(container)
	digest := hex.EncodeToString(sum[:])

	s := newServer(t)
	s.files["/downloads/abc-fw.cab"] = container
	s.files["/mirror/fw.cab"] = container

	fetcher := s.fetcher(pki, "firmware.xml.zst")

	t.Run("relative location", func(t *testing.T) {
		t.Parallel()

		release := &lvfs.Release{
			Locations: []string{"abc-fw.cab"},
			Checksums: []lvfs.Checksum{{Type: "sha256", Target: "container", Value: digest}},
		}

		got, err := fetcher.Release(context.Background(), release)
		require.NoError(t, err)
		assert.Equal(t, container, got)
	})

	t.Run("absolute location", func(t *testing.T) {
		t.Parallel()

		release := &lvfs.Release{
			Artifacts: []lvfs.Artifact{{
				Type:      "binary",
				Locations: []string{s.URL + "/mirror/fw.cab"},
				Checksums: []lvfs.Checksum{{Type: "sha256", Value: digest}},
			}},
		}

		got, err := fetcher.Release(context.Background(), release)
		require.NoError(t, err)
		assert.Equal(t, container, got)
	})

	t.Run("firmware base URL", func(t *testing.T) {
		t.Parallel()

		// the location in the metadata is unreachable; only its file name is used
		release := &lvfs.Release{
			Locations: []string{"https://lvfs.invalid/downloads/fw.cab"},
			Checksums: []lvfs.Checksum{{Type: "sha256", Target: "container", Value: digest}},
		}

		mirrored := *fetcher
		mirrored.FirmwareBaseURL = s.URL + "/mirror"

		got, err := mirrored.Release(context.Background(), release)
		require.NoError(t, err)
		assert.Equal(t, container, got)
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		t.Parallel()

		release := &lvfs.Release{
			Locations: []string{"abc-fw.cab"},
			Checksums: []lvfs.Checksum{{Type: "sha256", Target: "container", Value: "00" + digest[2:]}},
		}

		_, err := fetcher.Release(context.Background(), release)
		require.ErrorContains(t, err, "container abc-fw.cab: SHA-256 mismatch")
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		release := &lvfs.Release{
			Locations: []string{"missing.cab"},
			Checksums: []lvfs.Checksum{{Type: "sha256", Target: "container", Value: digest}},
		}

		_, err := fetcher.Release(context.Background(), release)
		require.ErrorContains(t, err, `error downloading missing.cab: unexpected status "404 Not Found"`)
	})

	t.Run("no location", func(t *testing.T) {
		t.Parallel()

		_, err := fetcher.Release(context.Background(), &lvfs.Release{})
		require.ErrorContains(t, err, "no download location")
	})

	t.Run("no checksum", func(t *testing.T) {
		t.Parallel()

		_, err := fetcher.Release(context.Background(), &lvfs.Release{Locations: []string{"abc-fw.cab"}})
		require.ErrorContains(t, err, "no container checksum")
	})
}

func TestFetcherDefaults(t *testing.T) {
	t.Parallel()

	// the zero Fetcher targets LVFS with the embedded CA; the request is
	// refused by the test transport, proving the defaults were used
	refused := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, lvfs.DefaultMetadataURL, r.URL.String())

		return nil, assert.AnError
	})}

	_, err := (&lvfs.Fetcher{Client: refused}).Metadata(context.Background())
	require.ErrorIs(t, err, assert.AnError)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
