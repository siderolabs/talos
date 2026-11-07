// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package manifestdownload_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	_ "embed"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/manifestdownload"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
)

const manifest = "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n"

// Checksums of the manifest which can't be computed in FIPS 140-only mode.
const (
	manifestMD5  = "d709d84f9d0d3a3bd7ab2e8feb1d75f6"
	manifestSHA1 = "18a9187f6d423e2fb955d1424f38370b0ed16792"
)

//go:embed testdata/manifest.yaml.bz2
var manifestBzip2 []byte

const manifestBzip2Contents = "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: bzip2\n"

type requestLog struct {
	mu       sync.Mutex
	requests []*http.Request
}

func (l *requestLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.requests = append(l.requests, r)
}

func (l *requestLog) last(t *testing.T) *http.Request {
	t.Helper()

	l.mu.Lock()
	defer l.mu.Unlock()

	require.NotEmpty(t, l.requests)

	return l.requests[len(l.requests)-1]
}

// serve starts a server which serves the given files by path.
func serve(t *testing.T, files map[string][]byte) (string, *requestLog) {
	t.Helper()

	log := &requestLog{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)

		data, ok := files[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Write(data) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)

	return srv.URL, log
}

func gzipData(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	return buf.Bytes()
}

func xzData(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	zw, err := xz.NewWriter(&buf)
	require.NoError(t, err)

	_, err = zw.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	return buf.Bytes()
}

func zstdData(t *testing.T, data []byte) []byte {
	t.Helper()

	zw, err := zstd.NewWriter(nil)
	require.NoError(t, err)

	defer zw.Close() //nolint:errcheck

	return zw.EncodeAll(data, nil)
}

type tarEntry struct {
	name     string
	typeflag byte
	data     string
}

func tarData(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	for _, entry := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     entry.name,
			Typeflag: entry.typeflag,
			Mode:     0o644,
			Size:     int64(len(entry.data)),
		}))

		_, err := tw.Write([]byte(entry.data))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())

	return buf.Bytes()
}

func zipData(t *testing.T, files ...string) []byte {
	t.Helper()

	var buf bytes.Buffer

	zw := zip.NewWriter(&buf)

	for i, name := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)

		if !strings.HasSuffix(name, "/") {
			_, err = w.Write([]byte(manifest[:len(manifest)-i]))
			require.NoError(t, err)
		}
	}

	require.NoError(t, zw.Close())

	return buf.Bytes()
}

func checksumHex(sum []byte) string {
	return hex.EncodeToString(sum)
}

func TestDownload(t *testing.T) {
	t.Parallel()

	singleTar := tarData(t, tarEntry{name: "manifest.yaml", typeflag: tar.TypeReg, data: manifest})
	sha256Sum := sha256.Sum256([]byte(manifest))
	sha512Sum := sha512.Sum512([]byte(manifest))
	gzipped := gzipData(t, []byte(manifest))
	gzippedSum := sha256.Sum256(gzipped)

	baseURL, log := serve(t, map[string][]byte{
		"/manifest.yaml":          []byte(manifest),
		"/manifest.yaml.gz":       gzipped,
		"/manifest.yaml.bz2":      manifestBzip2,
		"/manifest.yaml.xz":       xzData(t, []byte(manifest)),
		"/manifest.yaml.zst":      zstdData(t, []byte(manifest)),
		"/manifest.tar":           singleTar,
		"/manifest.tar.gz":        gzipData(t, singleTar),
		"/manifest.tgz":           gzipData(t, singleTar),
		"/manifest.tar.xz":        xzData(t, singleTar),
		"/manifest.txz":           xzData(t, singleTar),
		"/manifest.tar.zst":       zstdData(t, singleTar),
		"/manifest.tzst":          zstdData(t, singleTar),
		"/manifest.zip":           zipData(t, "manifest.yaml"),
		"/gzipped-no-suffix":      gzipped,
		"/plain.gz":               []byte(manifest),
		"/multi.tar":              tarData(t, tarEntry{name: "a.yaml", typeflag: tar.TypeReg, data: manifest}, tarEntry{name: "b.yaml", typeflag: tar.TypeReg, data: manifest}),
		"/dir.tar":                tarData(t, tarEntry{name: "dir/", typeflag: tar.TypeDir}),
		"/symlink.tar":            tarData(t, tarEntry{name: "link", typeflag: tar.TypeSymlink}),
		"/empty.tar":              tarData(t),
		"/multi.zip":              zipData(t, "a.yaml", "b.yaml"),
		"/dir.zip":                zipData(t, "dir/"),
		"/broken.gz":              []byte("not gzip"),
		"/broken.zip":             []byte("not zip"),
		"/manifest.yaml.tar.gz.d": []byte(manifest),
	})

	for _, test := range []struct {
		name string
		url  string

		expected      string
		expectedError string

		// fipsStrictError is the expected error in FIPS 140-only mode
		fipsStrictError string
	}{
		{name: "plain", url: "/manifest.yaml", expected: manifest},
		{name: "gz", url: "/manifest.yaml.gz", expected: manifest},
		{name: "bz2", url: "/manifest.yaml.bz2", expected: manifestBzip2Contents},
		{name: "xz", url: "/manifest.yaml.xz", expected: manifest},
		{name: "zst", url: "/manifest.yaml.zst", expected: manifest},
		{name: "tar", url: "/manifest.tar", expected: manifest},
		{name: "tar.gz", url: "/manifest.tar.gz", expected: manifest},
		{name: "tgz", url: "/manifest.tgz", expected: manifest},
		{name: "tar.xz", url: "/manifest.tar.xz", expected: manifest},
		{name: "txz", url: "/manifest.txz", expected: manifest},
		{name: "tar.zst", url: "/manifest.tar.zst", expected: manifest},
		{name: "tzst", url: "/manifest.tzst", expected: manifest},
		{name: "zip", url: "/manifest.zip", expected: manifest},
		{name: "suffix in the middle", url: "/manifest.yaml.tar.gz.d", expected: manifest},
		{name: "suffix in the query", url: "/manifest.yaml?file=manifest.yaml.gz", expected: manifest},
		{name: "archive override", url: "/gzipped-no-suffix?archive=gz", expected: manifest},
		{name: "archive disabled", url: "/plain.gz?archive=false", expected: manifest},
		{name: "unknown archive type", url: "/manifest.yaml?archive=rar", expected: manifest},

		{name: "checksum sha256", url: "/manifest.yaml?checksum=sha256:" + checksumHex(sha256Sum[:]), expected: manifest},
		{name: "checksum sha512", url: "/manifest.yaml?checksum=sha512:" + checksumHex(sha512Sum[:]), expected: manifest},
		// md5 and sha1 are precomputed, as they are not available in FIPS 140-only mode
		{
			name:            "checksum md5",
			url:             "/manifest.yaml?checksum=md5:" + manifestMD5,
			expected:        manifest,
			fipsStrictError: "md5 checksums are not allowed in FIPS 140-only mode",
		},
		{
			name:            "checksum md5 without type",
			url:             "/manifest.yaml?checksum=" + manifestMD5,
			expected:        manifest,
			fipsStrictError: "md5 checksums are not allowed in FIPS 140-only mode",
		},
		{
			name:            "checksum sha1",
			url:             "/manifest.yaml?checksum=sha1:" + manifestSHA1,
			expected:        manifest,
			fipsStrictError: "sha1 checksums are not allowed in FIPS 140-only mode",
		},
		{name: "checksum without type", url: "/manifest.yaml?checksum=" + checksumHex(sha256Sum[:]), expected: manifest},
		{name: "checksum of compressed content", url: "/manifest.yaml.gz?checksum=sha256:" + checksumHex(gzippedSum[:]), expected: manifest},

		{name: "forced getter", url: "https::" + "/manifest.yaml", expected: manifest},

		{
			name:          "checksum mismatch",
			url:           "/manifest.yaml?checksum=sha256:" + strings.Repeat("00", sha256.Size),
			expectedError: "checksum mismatch",
		},
		{name: "checksum file", url: "/manifest.yaml?checksum=file:http://example.com/SHA256SUMS", expectedError: "checksum files are not supported"},
		{name: "checksum unknown type", url: "/manifest.yaml?checksum=crc32:deadbeef", expectedError: "unsupported checksum type: crc32"},
		{name: "checksum wrong length", url: "/manifest.yaml?checksum=sha256:deadbeef", expectedError: "invalid sha256 checksum length: 4"},
		{name: "checksum not hex", url: "/manifest.yaml?checksum=sha256:zz", expectedError: "invalid checksum"},
		{name: "checksum unknown length", url: "/manifest.yaml?checksum=deadbeef", expectedError: "unknown type for checksum deadbeef"},

		{name: "not found", url: "/missing.yaml", expectedError: "bad response code: 404"},
		{name: "multiple files in tar", url: "/multi.tar", expectedError: "expected a single file, got multiple"},
		{name: "directory in tar", url: "/dir.tar", expectedError: `expected a single regular file, got "dir/"`},
		{name: "symlink in tar", url: "/symlink.tar", expectedError: `expected a single regular file, got "link"`},
		{name: "empty tar", url: "/empty.tar", expectedError: "empty archive"},
		{name: "multiple files in zip", url: "/multi.zip", expectedError: "expected a single file, got 2"},
		{name: "directory in zip", url: "/dir.zip", expectedError: `expected a single regular file, got "dir/"`},
		{name: "broken gzip", url: "/broken.gz", expectedError: `error decompressing "gz"`},
		{name: "broken zip", url: "/broken.zip", expectedError: `error decompressing "zip"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rawURL := baseURL + test.url
			if forced, rest, ok := strings.Cut(test.url, "::"); ok {
				rawURL = forced + "::" + baseURL + rest
			}

			data, err := manifestdownload.Download(t.Context(), http.DefaultClient, rawURL, nil)

			if fipsmode.Strict() && test.fipsStrictError != "" {
				test.expectedError = test.fipsStrictError
			}

			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, string(data))
		})
	}

	t.Run("magic query parameters are not sent", func(t *testing.T) {
		t.Parallel()

		_, err := manifestdownload.Download(t.Context(), http.DefaultClient,
			baseURL+"/manifest.yaml?b=2&checksum=sha256:"+checksumHex(sha256Sum[:])+"&archive=false&a=1", nil)
		require.NoError(t, err)

		var found bool

		log.mu.Lock()

		for _, r := range log.requests {
			if r.URL.Query().Get("b") == "2" && r.URL.Query().Get("a") == "1" {
				found = true

				assert.False(t, r.URL.Query().Has("checksum"))
				assert.False(t, r.URL.Query().Has("archive"))
			}
		}

		log.mu.Unlock()

		assert.True(t, found)
	})
}

func TestDownloadRequest(t *testing.T) {
	t.Parallel()

	baseURL, log := serve(t, map[string][]byte{
		"/manifest.yaml": []byte(manifest),
	})

	rawURL := baseURL + "/manifest.yaml?x=%2Fescaped&y=1"

	data, err := manifestdownload.Download(t.Context(), http.DefaultClient, rawURL, map[string]string{
		"Authorization": "Bearer token",
		"X-Custom":      "value",
	})
	require.NoError(t, err)
	assert.Equal(t, manifest, string(data))

	r := log.last(t)

	assert.Equal(t, http.MethodGet, r.Method)
	assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
	assert.Equal(t, "value", r.Header.Get("X-Custom"))

	// without magic query parameters, the query is passed as is
	assert.Equal(t, "x=%2Fescaped&y=1", r.URL.RawQuery)
}

func TestDownloadInvalidURL(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		url           string
		expectedError string
	}{
		{url: "ftp://example.com/manifest.yaml", expectedError: `unsupported URL scheme "ftp"`},
		{url: "file:///etc/passwd", expectedError: `unsupported URL scheme "file"`},
		{url: "/manifest.yaml", expectedError: `unsupported URL scheme ""`},
		{url: "git::https://example.com/repo.git", expectedError: `unsupported getter "git"`},
		{url: "s3::https://s3.amazonaws.com/bucket/manifest.yaml", expectedError: `unsupported getter "s3"`},
		{url: "https://example.com/%zz", expectedError: "error parsing URL"},
	} {
		t.Run(test.url, func(t *testing.T) {
			t.Parallel()

			_, err := manifestdownload.Download(t.Context(), http.DefaultClient, test.url, nil)
			require.ErrorContains(t, err, test.expectedError)
		})
	}
}

func TestDownloadSizeLimit(t *testing.T) {
	t.Parallel()

	// the bomb is small when compressed, but exceeds the limit when decompressed
	bomb := gzipData(t, make([]byte, manifestdownload.MaxSize+1))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/large.yaml":
			io.Copy(w, io.LimitReader(zeroReader{}, manifestdownload.MaxSize+1)) //nolint:errcheck
		case "/max.yaml":
			io.Copy(w, io.LimitReader(zeroReader{}, manifestdownload.MaxSize)) //nolint:errcheck
		case "/bomb.yaml.gz":
			w.Write(bomb) //nolint:errcheck
		case "/bomb.tar.gz":
			w.Write(gzipData(t, tarData(t, tarEntry{name: "a.yaml", typeflag: tar.TypeReg, data: string(make([]byte, manifestdownload.MaxSize+1))}))) //nolint:errcheck
		}
	}))
	t.Cleanup(srv.Close)

	_, err := manifestdownload.Download(t.Context(), http.DefaultClient, srv.URL+"/large.yaml", nil)
	require.ErrorContains(t, err, "content exceeds the size limit")

	data, err := manifestdownload.Download(t.Context(), http.DefaultClient, srv.URL+"/max.yaml", nil)
	require.NoError(t, err)
	assert.Len(t, data, manifestdownload.MaxSize)

	_, err = manifestdownload.Download(t.Context(), http.DefaultClient, srv.URL+"/bomb.yaml.gz", nil)
	require.ErrorContains(t, err, "content exceeds the size limit")

	_, err = manifestdownload.Download(t.Context(), http.DefaultClient, srv.URL+"/bomb.tar.gz", nil)
	require.ErrorContains(t, err, "content exceeds the size limit")
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)

	return len(p), nil
}
