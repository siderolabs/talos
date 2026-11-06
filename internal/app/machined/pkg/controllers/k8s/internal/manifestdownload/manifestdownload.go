// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package manifestdownload downloads Kubernetes manifests from HTTP(S) URLs.
//
// It keeps compatibility with the URL features previously provided by go-getter for single file downloads:
//
//   - forced getter prefix (http::/https::) is accepted and stripped;
//   - the "checksum" query parameter verifies the downloaded content ("type:hex" or just "hex");
//   - the content is decompressed based on the URL path suffix (.gz, .bz2, .xz, .zst, .tar, .tar.gz, .zip, etc.),
//     or based on the "archive" query parameter (which also allows to disable decompression with "archive=false");
//   - archives (tar, zip) must contain a single file.
//
// Both the downloaded and the decompressed content are limited to [MaxSize] bytes.
package manifestdownload

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/md5"  //nolint:gosec // md5 checksums are supported for compatibility
	"crypto/sha1" //nolint:gosec // sha1 checksums are supported for compatibility
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
)

// MaxSize is the maximum size of the downloaded and of the decompressed manifest.
const MaxSize = 64 << 20

// Download fetches the manifest from the URL with the given HTTP client and headers.
func Download(ctx context.Context, client *http.Client, rawURL string, headers map[string]string) ([]byte, error) {
	src, err := parseSource(rawURL)
	if err != nil {
		return nil, err
	}

	data, err := fetch(ctx, client, src.url, headers)
	if err != nil {
		return nil, err
	}

	if src.checksum != nil {
		if err = src.checksum.verify(data); err != nil {
			return nil, err
		}
	}

	if src.archive == "" {
		return data, nil
	}

	data, err = decompress(src.archive, data)
	if err != nil {
		return nil, fmt.Errorf("error decompressing %q: %w", src.archive, err)
	}

	return data, nil
}

func fetch(ctx context.Context, client *http.Client, u *url.URL, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response code: %d", resp.StatusCode)
	}

	data, err := readLimited(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	return data, nil
}

type source struct {
	url      *url.URL
	checksum *checksum
	archive  string
}

// Compression formats.
const (
	compressionBzip2 = "bz2"
	compressionGzip  = "gz"
	compressionXz    = "xz"
	compressionZstd  = "zst"
)

type archiveFormat struct {
	compression string
	tar         bool
	zip         bool
}

// archiveFormats lists the supported archive types, which are matched against the URL path suffix.
var archiveFormats = map[string]archiveFormat{
	"bz2":     {compression: compressionBzip2},
	"gz":      {compression: compressionGzip},
	"xz":      {compression: compressionXz},
	"zst":     {compression: compressionZstd},
	"tar":     {tar: true},
	"tar.bz2": {compression: compressionBzip2, tar: true},
	"tar.gz":  {compression: compressionGzip, tar: true},
	"tar.xz":  {compression: compressionXz, tar: true},
	"tar.zst": {compression: compressionZstd, tar: true},
	"tbz2":    {compression: compressionBzip2, tar: true},
	"tgz":     {compression: compressionGzip, tar: true},
	"txz":     {compression: compressionXz, tar: true},
	"tzst":    {compression: compressionZstd, tar: true},
	"zip":     {zip: true},
}

func parseSource(rawURL string) (*source, error) {
	rawURL, err := stripForcedGetter(rawURL)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}

	src := &source{
		url:     u,
		archive: matchArchiveType(u.Path),
	}

	if err = src.parseMagicQuery(); err != nil {
		return nil, err
	}

	return src, nil
}

// parseMagicQuery handles the go-getter magic query parameters, removing them from the URL.
func (src *source) parseMagicQuery() error {
	query := src.url.Query()

	if !query.Has("checksum") && !query.Has("archive") {
		// no magic query parameters, so the URL is used as is
		return nil
	}

	if v := query.Get("checksum"); v != "" {
		var err error

		src.checksum, err = parseChecksum(v)
		if err != nil {
			return fmt.Errorf("invalid checksum: %w", err)
		}
	}

	if v := query.Get("archive"); v != "" {
		src.archive = ""

		// "archive=false" disables decompression, unknown archive types (including "true") don't enable it
		if _, ok := archiveFormats[v]; ok {
			src.archive = v
		}
	}

	// these query parameters are not passed to the server
	query.Del("checksum")
	query.Del("archive")

	src.url.RawQuery = query.Encode()

	return nil
}

// stripForcedGetter strips the go-getter forced getter prefix, e.g. https::https://example.com/manifest.yaml.
func stripForcedGetter(rawURL string) (string, error) {
	forced, rest, found := strings.Cut(rawURL, "::")
	if !found || strings.Contains(forced, "/") {
		return rawURL, nil
	}

	if forced != "http" && forced != "https" {
		return "", fmt.Errorf("unsupported getter %q", forced)
	}

	return rest, nil
}

// matchArchiveType returns the longest archive type which matches the path as a file extension.
func matchArchiveType(path string) string {
	var match string

	for archiveType := range archiveFormats {
		if strings.HasSuffix(path, "."+archiveType) && len(archiveType) > len(match) {
			match = archiveType
		}
	}

	return match
}

type checksum struct {
	newHash func() hash.Hash
	value   []byte
}

// fipsApprovedChecksums lists the checksum types allowed in FIPS 140-only mode.
var fipsApprovedChecksums = map[string]bool{
	"sha256": true,
	"sha512": true,
}

// checksumTypes lists the supported checksum types.
var checksumTypes = map[string]func() hash.Hash{
	"md5":    md5.New,
	"sha1":   sha1.New,
	"sha256": sha256.New,
	"sha512": sha512.New,
}

func parseChecksum(v string) (*checksum, error) {
	checksumType, checksumValue, found := strings.Cut(v, ":")
	if !found {
		checksumType, checksumValue = "", v
	}

	if checksumType == "file" {
		return nil, errors.New("checksum files are not supported")
	}

	value, err := hex.DecodeString(checksumValue)
	if err != nil {
		return nil, err
	}

	if checksumType == "" {
		if checksumType, err = guessChecksumType(value); err != nil {
			return nil, fmt.Errorf("%w %s", err, checksumValue)
		}
	}

	newHash, ok := checksumTypes[checksumType]
	if !ok {
		return nil, fmt.Errorf("unsupported checksum type: %s", checksumType)
	}

	if fipsmode.Strict() && !fipsApprovedChecksums[checksumType] {
		// the hash functions panic when used in FIPS 140-only mode
		return nil, fmt.Errorf("%s checksums are not allowed in FIPS 140-only mode", checksumType)
	}

	if size := newHash().Size(); size != len(value) {
		return nil, fmt.Errorf("invalid %s checksum length: %d", checksumType, len(value))
	}

	return &checksum{
		newHash: newHash,
		value:   value,
	}, nil
}

// guessChecksumType guesses the type from the length of the checksum (all supported types have different lengths).
func guessChecksumType(value []byte) (string, error) {
	for name, newHash := range checksumTypes {
		if newHash().Size() == len(value) {
			return name, nil
		}
	}

	return "", errors.New("unknown type for checksum")
}

func (c *checksum) verify(data []byte) error {
	h := c.newHash()
	h.Write(data) //nolint:errcheck

	if actual := h.Sum(nil); subtle.ConstantTimeCompare(actual, c.value) != 1 {
		return fmt.Errorf("checksum mismatch: expected %x, got %x", c.value, actual)
	}

	return nil
}

// readLimited reads the whole reader, failing if it is larger than MaxSize.
func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxSize+1))
	if err != nil {
		return nil, err
	}

	if len(data) > MaxSize {
		return nil, fmt.Errorf("content exceeds the size limit of %d bytes", MaxSize)
	}

	return data, nil
}

func decompress(archiveType string, data []byte) ([]byte, error) {
	format := archiveFormats[archiveType]

	if format.zip {
		return unzipSingleFile(data)
	}

	r, closer, err := decompressStream(format.compression, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	if closer != nil {
		defer closer()
	}

	if format.tar {
		return untarSingleFile(r)
	}

	return readLimited(r)
}

func decompressStream(compression string, r io.Reader) (io.Reader, func(), error) {
	switch compression {
	case "":
		return r, nil, nil
	case compressionBzip2:
		return bzip2.NewReader(r), nil, nil
	case compressionGzip:
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, err
		}

		return zr, func() { zr.Close() }, nil //nolint:errcheck
	case compressionXz:
		zr, err := xz.NewReader(r)
		if err != nil {
			return nil, nil, err
		}

		return zr, nil, nil
	case compressionZstd:
		zr, err := zstd.NewReader(r, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(MaxSize))
		if err != nil {
			return nil, nil, err
		}

		return zr, zr.Close, nil
	default:
		return nil, nil, fmt.Errorf("unsupported compression %q", compression)
	}
}

func untarSingleFile(r io.Reader) ([]byte, error) {
	tr := tar.NewReader(r)

	var data []byte

	found := false

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, err
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		case tar.TypeReg:
		default:
			return nil, fmt.Errorf("expected a single regular file, got %q", hdr.Name)
		}

		if found {
			return nil, errors.New("expected a single file, got multiple")
		}

		found = true

		if data, err = readLimited(tr); err != nil {
			return nil, err
		}
	}

	if !found {
		return nil, errors.New("empty archive")
	}

	return data, nil
}

func unzipSingleFile(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	if len(zr.File) != 1 {
		return nil, fmt.Errorf("expected a single file, got %d", len(zr.File))
	}

	f := zr.File[0]

	if !f.Mode().IsRegular() {
		return nil, fmt.Errorf("expected a single regular file, got %q", f.Name)
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}

	defer rc.Close() //nolint:errcheck

	return readLimited(rc)
}
