// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package jcat_test

import (
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/fips140"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/jcat"
)

type testPKI struct {
	roots      *x509.CertPool
	signerCert *x509.Certificate
	signerKey  *ecdsa.PrivateKey
}

func newTestPKI(t *testing.T) *testPKI {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	signerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	signerTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Test Signer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	signerDER, err := x509.CreateCertificate(rand.Reader, signerTemplate, caCert, &signerKey.PublicKey, caKey)
	require.NoError(t, err)

	signerCert, err := x509.ParseCertificate(signerDER)
	require.NoError(t, err)

	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	return &testPKI{
		roots:      roots,
		signerCert: signerCert,
		signerKey:  signerKey,
	}
}

func (pki *testPKI) sign(t *testing.T, payload []byte) []byte {
	t.Helper()

	var signature []byte

	// pkcs7 signing hits SHA-1 internally for signer-identity fields; bypass
	// FIPS enforcement so this test fixture can be generated in FIPS-strict builds.
	fips140.WithoutEnforcement(func() {
		sd, err := pkcs7.NewSignedData(payload)
		require.NoError(t, err)

		require.NoError(t, sd.AddSigner(pki.signerCert, pki.signerKey, pkcs7.SignerInfoConfig{}))

		sd.Detach()

		signature, err = sd.Finish()
		require.NoError(t, err)
	})

	return signature
}

func buildJcat(t *testing.T, itemID string, aliasIDs []string, blobs []jcat.Blob) []byte {
	t.Helper()

	doc := map[string]any{
		"JcatVersionMajor": 0,
		"JcatVersionMinor": 1,
		"Items": []map[string]any{
			{
				"Id":       itemID,
				"AliasIds": aliasIDs,
				"Blobs":    blobs,
			},
		},
	}

	encoded, err := json.Marshal(doc)
	require.NoError(t, err)

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)

	_, err = gz.Write(encoded)
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	// libjcat-produced files carry trailing bytes after the gzip member
	buf.Write([]byte{0xde, 0xad, 0xbe, 0xef})

	return buf.Bytes()
}

func TestVerify(t *testing.T) {
	t.Parallel()

	pki := newTestPKI(t)

	payload := []byte("firmware metadata payload")
	sum := sha256.Sum256(payload)

	signature := pki.sign(t, payload)

	jcatData := buildJcat(t, "firmware-12345-stable.xml.zst", []string{"firmware.xml.zst"}, []jcat.Blob{
		{Kind: jcat.KindSHA256, Flags: 1, Data: hex.EncodeToString(sum[:])},
		{Kind: jcat.KindPKCS7, Data: base64.StdEncoding.EncodeToString(signature)},
	})

	f, err := jcat.Parse(bytes.NewReader(jcatData))
	require.NoError(t, err)

	_, ok := f.Item("nonexistent")
	assert.False(t, ok)

	item, ok := f.Item("firmware.xml.zst")
	require.True(t, ok)

	assert.NoError(t, item.Verify(payload, pki.roots))

	assert.ErrorContains(t, item.Verify([]byte("tampered payload"), pki.roots), "checksum mismatch")

	otherPKI := newTestPKI(t)
	assert.ErrorContains(t, item.Verify(payload, otherPKI.roots), "no valid PKCS7 signature")
}

func TestVerifyNoSignature(t *testing.T) {
	t.Parallel()

	pki := newTestPKI(t)

	payload := []byte("payload")
	sum := sha256.Sum256(payload)

	jcatData := buildJcat(t, "firmware.xml.zst", nil, []jcat.Blob{
		{Kind: jcat.KindSHA256, Flags: 1, Data: hex.EncodeToString(sum[:])},
	})

	f, err := jcat.Parse(bytes.NewReader(jcatData))
	require.NoError(t, err)

	item, ok := f.Item("firmware.xml.zst")
	require.True(t, ok)

	assert.ErrorContains(t, item.Verify(payload, pki.roots), "no PKCS7 signatures found")
}
