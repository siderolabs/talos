// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package jcat_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/jcat"
	"github.com/siderolabs/talos/internal/pkg/firmware/jcat/jcattest"
)

func TestVerify(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)

	payload := []byte("firmware metadata payload")

	jcatData := jcattest.Build(t, pki.Item(t, "firmware-12345-stable.xml.zst", []string{"firmware.xml.zst"}, payload))

	f, err := jcat.Parse(bytes.NewReader(jcatData))
	require.NoError(t, err)

	_, ok := f.Item("nonexistent")
	assert.False(t, ok)

	item, ok := f.Item("firmware.xml.zst")
	require.True(t, ok)

	assert.NoError(t, item.Verify(payload, pki.Roots))

	assert.ErrorContains(t, item.Verify([]byte("tampered payload"), pki.Roots), "checksum mismatch")

	otherPKI := jcattest.NewPKI(t)
	assert.ErrorContains(t, item.Verify(payload, otherPKI.Roots), "no valid PKCS7 signature")
}

func TestParseSizeLimit(t *testing.T) {
	t.Parallel()

	// valid JSON padded past the limit compresses to a few kilobytes
	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)

	_, err := gz.Write([]byte(`{"JcatVersionMajor":0,"JcatVersionMinor":1,"Items":[]}`))
	require.NoError(t, err)

	_, err = gz.Write(bytes.Repeat([]byte(" "), jcat.MaxSize))
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	require.Less(t, buf.Len(), 1<<20)

	_, err = jcat.Parse(&buf)
	require.ErrorContains(t, err, "expands to more than")
}

func TestParseTruncated(t *testing.T) {
	t.Parallel()

	data := jcattest.Build(t, jcat.Item{ID: "firmware.bin"})

	_, err := jcat.Parse(bytes.NewReader(data))
	require.NoError(t, err)

	// cut into the gzip trailer; Build appends 4 bytes after the member
	_, err = jcat.Parse(bytes.NewReader(data[:len(data)-8]))
	require.ErrorContains(t, err, "error reading jcat gzip stream")
}

func TestVerifyNoSignature(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)

	payload := []byte("payload")
	sum := sha256.Sum256(payload)

	jcatData := jcattest.Build(t, jcat.Item{
		ID: "firmware.xml.zst",
		Blobs: []jcat.Blob{
			{Kind: jcat.KindSHA256, Flags: 1, Data: hex.EncodeToString(sum[:])},
		},
	})

	f, err := jcat.Parse(bytes.NewReader(jcatData))
	require.NoError(t, err)

	item, ok := f.Item("firmware.xml.zst")
	require.True(t, ok)

	assert.ErrorContains(t, item.Verify(payload, pki.Roots), "no PKCS7 signatures found")
	assert.NotErrorIs(t, item.Verify(payload, pki.Roots), jcat.ErrUnsupportedSignature)
}

func TestVerifyUnsupportedSignature(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)

	payload := []byte("payload")

	// an item signed only with GPG is told apart from one whose signature failed
	gpgOnly := jcat.Item{
		ID:    "firmware.xml.zst",
		Blobs: []jcat.Blob{{Kind: jcat.KindGPG, Flags: 1, Data: "-----BEGIN PGP SIGNATURE-----"}},
	}

	assert.ErrorIs(t, gpgOnly.Verify(payload, pki.Roots), jcat.ErrUnsupportedSignature)

	// a PKCS7 signature that fails is reported as such, GPG blob or not
	both := jcat.Item{
		ID: "firmware.xml.zst",
		Blobs: []jcat.Blob{
			{Kind: jcat.KindGPG, Flags: 1, Data: "-----BEGIN PGP SIGNATURE-----"},
			{Kind: jcat.KindPKCS7, Data: "not valid base64!"},
		},
	}

	err := both.Verify(payload, pki.Roots)
	require.ErrorContains(t, err, "error decoding blob data")
	assert.NotErrorIs(t, err, jcat.ErrUnsupportedSignature)

	// and a valid PKCS7 signature is enough alongside a GPG one
	signed := pki.Item(t, "firmware.xml.zst", nil, payload)
	signed.Blobs = append(signed.Blobs, jcat.Blob{Kind: jcat.KindGPG, Flags: 1, Data: "-----BEGIN PGP SIGNATURE-----"})

	assert.NoError(t, signed.Verify(payload, pki.Roots))
}

func TestVerifyMalformedSignatureBlob(t *testing.T) {
	t.Parallel()

	pki := jcattest.NewPKI(t)

	payload := []byte("payload")
	sum := sha256.Sum256(payload)

	signature := pki.Sign(t, payload)

	// a malformed PKCS7 blob must not prevent a later valid signature from verifying
	jcatData := jcattest.Build(t, jcat.Item{
		ID: "firmware.xml.zst",
		Blobs: []jcat.Blob{
			{Kind: jcat.KindSHA256, Flags: 1, Data: hex.EncodeToString(sum[:])},
			{Kind: jcat.KindPKCS7, Data: "not valid base64!"},
			{Kind: jcat.KindPKCS7, Data: base64.StdEncoding.EncodeToString(signature)},
		},
	})

	f, err := jcat.Parse(bytes.NewReader(jcatData))
	require.NoError(t, err)

	item, ok := f.Item("firmware.xml.zst")
	require.True(t, ok)

	assert.NoError(t, item.Verify(payload, pki.Roots))

	// with only the malformed blob, verification fails
	jcatData = jcattest.Build(t, jcat.Item{
		ID: "firmware.xml.zst",
		Blobs: []jcat.Blob{
			{Kind: jcat.KindSHA256, Flags: 1, Data: hex.EncodeToString(sum[:])},
			{Kind: jcat.KindPKCS7, Data: "not valid base64!"},
		},
	})

	f, err = jcat.Parse(bytes.NewReader(jcatData))
	require.NoError(t, err)

	item, ok = f.Item("firmware.xml.zst")
	require.True(t, ok)

	assert.ErrorContains(t, item.Verify(payload, pki.Roots), "error decoding blob data")
}
