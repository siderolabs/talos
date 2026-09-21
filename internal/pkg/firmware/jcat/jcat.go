// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package jcat implements parsing and verification of libjcat signature files.
//
// A jcat file is a gzip-compressed JSON document listing items (by filename)
// with checksum and signature blobs, as produced by libjcat and used by LVFS
// to sign firmware metadata and payloads.
package jcat

import (
	"compress/gzip"
	"crypto/fips140"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/digitorus/pkcs7"
)

// Blob kinds (libjcat JcatBlobKind values).
const (
	KindSHA256 = 1
	KindGPG    = 2
	KindPKCS7  = 3
	KindSHA1   = 4
)

const flagIsUTF8 = 1

// File is a parsed jcat file.
type File struct {
	Items []Item `json:"Items"`
}

// Item describes signatures and checksums for a single file.
type Item struct {
	ID       string   `json:"Id"`
	AliasIDs []string `json:"AliasIds"`
	Blobs    []Blob   `json:"Blobs"`
}

// Blob is a single checksum or signature.
type Blob struct {
	Kind  int    `json:"Kind"`
	Flags int    `json:"Flags"`
	Data  string `json:"Data"`
}

// Bytes returns the decoded blob data.
func (b Blob) Bytes() ([]byte, error) {
	if b.Flags&flagIsUTF8 != 0 {
		return []byte(b.Data), nil
	}

	return base64.StdEncoding.DecodeString(b.Data)
}

// Parse reads a gzip-compressed jcat file.
func Parse(r io.Reader) (*File, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("error reading jcat gzip stream: %w", err)
	}

	defer gz.Close() //nolint:errcheck

	// jcat files may have trailing bytes after the gzip member
	gz.Multistream(false)

	var f File

	if err := json.NewDecoder(gz).Decode(&f); err != nil {
		return nil, fmt.Errorf("error decoding jcat JSON: %w", err)
	}

	// drain to force the gzip CRC/length trailer to be validated; json.Decoder
	// may return as soon as the top-level value is parsed and leave the trailer
	// unread, which would silently accept a truncated stream.
	if _, err := io.Copy(io.Discard, gz); err != nil {
		return nil, fmt.Errorf("error validating jcat gzip trailer: %w", err)
	}

	return &f, nil
}

// Item looks up an item by ID or alias ID.
func (f *File) Item(id string) (*Item, bool) {
	for i, item := range f.Items {
		if item.ID == id || slices.Contains(item.AliasIDs, id) {
			return &f.Items[i], true
		}
	}

	return nil, false
}

// Verify checks the payload against the item: the SHA-256 checksum blob (if present)
// must match, and at least one PKCS7 signature must verify with a chain to roots.
//
//nolint:gocyclo
func (i *Item) Verify(payload []byte, roots *x509.CertPool) error {
	var (
		signatureValid bool
		errs           error
	)

	for _, blob := range i.Blobs {
		switch blob.Kind {
		case KindSHA256:
			// libjcat stores checksum digests as hex strings, regardless of the
			// UTF8 flag; do not base64-decode.
			sum := sha256.Sum256(payload)

			if !strings.EqualFold(strings.TrimSpace(blob.Data), hex.EncodeToString(sum[:])) {
				return fmt.Errorf("SHA-256 checksum mismatch for %q", i.ID)
			}
		case KindPKCS7:
			data, err := blob.Bytes()
			if err != nil {
				return fmt.Errorf("error decoding blob data: %w", err)
			}

			if err := verifyPKCS7(data, payload, roots); err != nil {
				errs = errors.Join(errs, err)

				continue
			}

			signatureValid = true
		}
	}

	if !signatureValid {
		if errs == nil {
			errs = errors.New("no PKCS7 signatures found")
		}

		return fmt.Errorf("no valid PKCS7 signature for %q: %w", i.ID, errs)
	}

	return nil
}

// verifyPKCS7 parses and verifies a PKCS7 signature over payload against roots.
//
// The signature verification path in digitorus/pkcs7 uses SHA-1 for a few
// internal certificate-identity computations (SKI/AKI, IssuerAndSerialNumber
// hash) even when the actual message digest is SHA-256; bypass FIPS
// enforcement so LVFS signatures — which are already scoped to firmware
// metadata authenticity rather than a Talos security boundary — verify in
// FIPS-strict builds.
func verifyPKCS7(data, payload []byte, roots *x509.CertPool) error {
	// blobs may be PEM-encoded or raw DER
	if block, _ := pem.Decode(data); block != nil {
		data = block.Bytes
	}

	var out error

	fips140.WithoutEnforcement(func() {
		p7, err := pkcs7.Parse(data)
		if err != nil {
			out = fmt.Errorf("error parsing PKCS7 signature: %w", err)

			return
		}

		p7.Content = payload

		if err := p7.VerifyWithChain(roots); err != nil {
			out = fmt.Errorf("error verifying PKCS7 signature: %w", err)
		}
	})

	return out
}
