// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package jcattest builds signed jcat files for tests.
package jcattest

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
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/jcat"
)

// PKI is a throwaway CA with one signer certificate.
type PKI struct {
	// Roots trusts the CA.
	Roots *x509.CertPool

	signerCert *x509.Certificate
	signerKey  *ecdsa.PrivateKey
}

// NewPKI generates a CA and a signer certificate issued by it.
func NewPKI(t *testing.T) *PKI {
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

	return &PKI{
		Roots:      roots,
		signerCert: signerCert,
		signerKey:  signerKey,
	}
}

// Sign returns a detached PKCS7 signature of the payload.
func (pki *PKI) Sign(t *testing.T, payload []byte) []byte {
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

// Item describes a payload with its SHA-256 checksum and a PKCS7 signature,
// the way LVFS signs files.
func (pki *PKI) Item(t *testing.T, id string, aliasIDs []string, payload []byte) jcat.Item {
	t.Helper()

	sum := sha256.Sum256(payload)

	return jcat.Item{
		ID:       id,
		AliasIDs: aliasIDs,
		Blobs: []jcat.Blob{
			{Kind: jcat.KindSHA256, Flags: 1, Data: hex.EncodeToString(sum[:])},
			{Kind: jcat.KindPKCS7, Data: base64.StdEncoding.EncodeToString(pki.Sign(t, payload))},
		},
	}
}

// Build encodes items as a jcat file.
func Build(t *testing.T, items ...jcat.Item) []byte {
	t.Helper()

	doc := struct {
		VersionMajor int         `json:"JcatVersionMajor"`
		VersionMinor int         `json:"JcatVersionMinor"`
		Items        []jcat.Item `json:"Items"`
	}{
		VersionMinor: 1,
		Items:        items,
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
