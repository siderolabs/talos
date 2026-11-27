// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"github.com/siderolabs/talos/internal/pkg/firmware/jcat"
)

// DefaultMetadataURL is the stable LVFS metadata.
const DefaultMetadataURL = "https://cdn.fwupd.org/downloads/firmware.xml.zst"

const (
	// MaxMetadataSize caps a metadata download, before and after decompression.
	MaxMetadataSize = 256 << 20
	// MaxReleaseSize caps a release container download.
	MaxReleaseSize = 256 << 20

	// maxSignatureSize caps a jcat download.
	maxSignatureSize = 1 << 20

	// signatureSuffix is appended to the metadata URL to locate its signature.
	signatureSuffix = ".jcat"
)

// Fetcher downloads LVFS metadata and release containers.
type Fetcher struct {
	// Client is the HTTP client; http.DefaultClient when nil.
	Client *http.Client
	// MetadataURL locates the metadata; DefaultMetadataURL when empty.
	// Relative release locations resolve against it.
	MetadataURL string
	// FirmwareBaseURL, when set, is where release containers are downloaded
	// from, by file name, instead of the locations in the metadata. A mirror
	// needs it, as the signed metadata points at LVFS with absolute URLs.
	FirmwareBaseURL string
	// Roots are the trusted metadata signers; the embedded LVFS CA when nil.
	Roots *x509.CertPool
}

// Metadata downloads the metadata and its detached jcat signature, verifies
// the signature, and returns the decompressed AppStream XML.
func (f *Fetcher) Metadata(ctx context.Context) ([]byte, error) {
	metadataURL := f.metadataURL()

	name, signatureURL, err := metadataNames(metadataURL)
	if err != nil {
		return nil, err
	}

	compressed, err := f.get(ctx, metadataURL, MaxMetadataSize)
	if err != nil {
		return nil, fmt.Errorf("error downloading metadata: %w", err)
	}

	signature, err := f.get(ctx, signatureURL, maxSignatureSize)
	if err != nil {
		return nil, fmt.Errorf("error downloading metadata signature: %w", err)
	}

	if err = verify(signature, name, compressed, f.roots()); err != nil {
		return nil, fmt.Errorf("metadata signature: %w", err)
	}

	data, err := decompress(name, compressed, MaxMetadataSize)
	if err != nil {
		return nil, fmt.Errorf("error decompressing metadata: %w", err)
	}

	return data, nil
}

// Release downloads the container of a release and verifies it against the
// checksum the catalog carries for it.
func (f *Fetcher) Release(ctx context.Context, release *Release) ([]byte, error) {
	location, sum := release.Cab()

	if location == "" {
		return nil, errors.New("release has no download location")
	}

	if sum == "" {
		return nil, errors.New("release has no container checksum")
	}

	target, err := f.releaseURL(location)
	if err != nil {
		return nil, err
	}

	data, err := f.get(ctx, target, MaxReleaseSize)
	if err != nil {
		return nil, fmt.Errorf("error downloading %s: %w", location, err)
	}

	if err = checkSHA256(data, sum); err != nil {
		return nil, fmt.Errorf("container %s: %w", location, err)
	}

	return data, nil
}

// releaseURL resolves a release location: against the metadata URL, or to
// the file of the same name under FirmwareBaseURL when that is set.
func (f *Fetcher) releaseURL(location string) (string, error) {
	ref, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("invalid release location %q: %w", location, err)
	}

	if f.FirmwareBaseURL != "" {
		base, err := url.Parse(f.FirmwareBaseURL)
		if err != nil {
			return "", fmt.Errorf("invalid firmware base URL: %w", err)
		}

		name := path.Base(ref.Path)
		if name == "." || name == "/" {
			return "", fmt.Errorf("release location %q has no file name", location)
		}

		return base.JoinPath(name).String(), nil
	}

	base, err := url.Parse(f.metadataURL())
	if err != nil {
		return "", fmt.Errorf("invalid metadata URL: %w", err)
	}

	return base.ResolveReference(ref).String(), nil
}

func (f *Fetcher) metadataURL() string {
	if f.MetadataURL == "" {
		return DefaultMetadataURL
	}

	return f.MetadataURL
}

func (f *Fetcher) roots() *x509.CertPool {
	if f.Roots == nil {
		return CertPool()
	}

	return f.Roots
}

// get downloads a URL, failing on any status but 200 OK and on bodies over limit bytes.
func (f *Fetcher) get(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %q", resp.Status)
	}

	return readLimited(resp.Body, limit)
}

// metadataNames returns the last path element of the metadata URL, which
// names the metadata in its jcat file and carries the compression suffix, and
// the URL of that jcat file: the same URL with the suffix added to its path,
// keeping any query.
func metadataNames(rawURL string) (name, signatureURL string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid metadata URL: %w", err)
	}

	name = path.Base(u.Path)
	if name == "." || name == "/" {
		return "", "", fmt.Errorf("metadata URL %q has no file name", rawURL)
	}

	u.Path += signatureSuffix

	if u.RawPath != "" {
		u.RawPath += signatureSuffix
	}

	return name, u.String(), nil
}

// verify checks a payload against its item in a jcat signature file.
func verify(signature []byte, name string, payload []byte, roots *x509.CertPool) error {
	jf, err := jcat.Parse(bytes.NewReader(signature))
	if err != nil {
		return fmt.Errorf("error parsing jcat: %w", err)
	}

	item, ok := jf.Item(name)
	if !ok {
		return fmt.Errorf("no jcat item for %q", name)
	}

	return item.Verify(payload, roots)
}

// decompress decodes data by the compression suffix of its name.
func decompress(name string, data []byte, limit int64) ([]byte, error) {
	var r io.Reader

	switch path.Ext(name) {
	case ".zst":
		zr, err := zstd.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}

		defer zr.Close()

		r = zr
	case ".xz":
		xr, err := xz.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}

		r = xr
	case ".gz":
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}

		defer gr.Close() //nolint:errcheck

		r = gr
	default:
		return data, nil
	}

	return readLimited(r, limit)
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, fmt.Errorf("content exceeds the size limit of %d bytes", limit)
	}

	return data, nil
}

// checkSHA256 compares data against a hex-encoded SHA-256 digest.
func checkSHA256(data []byte, expected string) error {
	sum := sha256.Sum256(data)

	if actual := hex.EncodeToString(sum[:]); !strings.EqualFold(actual, expected) {
		return fmt.Errorf("SHA-256 mismatch: got %s, expected %s", actual, expected)
	}

	return nil
}
