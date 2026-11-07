// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cosign_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"maps"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/tls"
	ctx509 "github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ourcosign "github.com/siderolabs/talos/internal/pkg/containers/image/verify/internal/cosign"
)

// Legacy cosign signature layer annotations.
const (
	signatureAnnotation   = "dev.cosignproject.cosign/signature"
	certificateAnnotation = "dev.sigstore.cosign/certificate"
	chainAnnotation       = "dev.sigstore.cosign/chain"
	bundleAnnotation      = "dev.sigstore.cosign/bundle"
)

// testSigstore is a self-contained Sigstore instance: a Fulcio CA (root and intermediate),
// a certificate transparency log and a Rekor transparency log.
//
// It issues short-lived signing certificates with an embedded SCT and signs Rekor entries
// (with a signed entry timestamp), i.e. everything a legacy keyless cosign signature carries.
// It also serves as the trusted material for verification.
type testSigstore struct {
	root.BaseTrustedMaterial

	rootCert         *x509.Certificate
	intermediateCert *x509.Certificate
	intermediateKey  *ecdsa.PrivateKey

	ctlogKey *ecdsa.PrivateKey
	ctlogID  [32]byte

	rekorKey           *ecdsa.PrivateKey
	rekorLogID         [32]byte
	rekorValidityStart time.Time
}

func newTestSigstore(t *testing.T) *testSigstore {
	t.Helper()

	now := time.Now()

	ts := &testSigstore{
		rekorValidityStart: now.Add(-time.Hour),
	}

	caTemplate := func(name string) *x509.Certificate {
		return &x509.Certificate{
			SerialNumber:          newSerialNumber(t),
			Subject:               pkix.Name{CommonName: name},
			NotBefore:             now.Add(-time.Hour),
			NotAfter:              now.Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
		}
	}

	rootKey := newKey(t)
	rootTemplate := caTemplate("test root")
	ts.rootCert = createCertificate(t, rootTemplate, rootTemplate, rootKey, &rootKey.PublicKey)

	ts.intermediateKey = newKey(t)
	ts.intermediateCert = createCertificate(t, caTemplate("test intermediate"), ts.rootCert, rootKey, &ts.intermediateKey.PublicKey)

	ts.ctlogKey = newKey(t)
	ts.ctlogID = logID(t, &ts.ctlogKey.PublicKey)

	ts.rekorKey = newKey(t)
	ts.rekorLogID = logID(t, &ts.rekorKey.PublicKey)

	return ts
}

func (ts *testSigstore) FulcioCertificateAuthorities() []root.CertificateAuthority {
	return []root.CertificateAuthority{
		&root.FulcioCertificateAuthority{
			Root:          ts.rootCert,
			Intermediates: []*x509.Certificate{ts.intermediateCert},
		},
	}
}

func (ts *testSigstore) CTLogs() map[string]*root.TransparencyLog {
	return map[string]*root.TransparencyLog{
		hex.EncodeToString(ts.ctlogID[:]): {
			ID:                ts.ctlogID[:],
			HashFunc:          crypto.SHA256,
			PublicKey:         &ts.ctlogKey.PublicKey,
			SignatureHashFunc: crypto.SHA256,
		},
	}
}

func (ts *testSigstore) RekorLogs() map[string]*root.TransparencyLog {
	return map[string]*root.TransparencyLog{
		hex.EncodeToString(ts.rekorLogID[:]): {
			BaseURL:             "https://rekor.test",
			ID:                  ts.rekorLogID[:],
			ValidityPeriodStart: ts.rekorValidityStart,
			HashFunc:            crypto.SHA256,
			PublicKey:           &ts.rekorKey.PublicKey,
			SignatureHashFunc:   crypto.SHA256,
		},
	}
}

// issueCertificate issues a short-lived Fulcio-style signing certificate for the identity.
//
// With embedSCT, the certificate carries an SCT signed by the certificate transparency log,
// built the same way Fulcio does it: the SCT signs the pre-certificate (the certificate without
// the SCT extension).
func (ts *testSigstore) issueCertificate(t *testing.T, subject, issuer string, embedSCT bool) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key := newKey(t)
	now := time.Now()

	template := &x509.Certificate{
		SerialNumber:   newSerialNumber(t),
		EmailAddresses: []string{subject},
		NotBefore:      now.Add(-time.Minute),
		NotAfter:       now.Add(10 * time.Minute),
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		ExtraExtensions: []pkix.Extension{
			{
				// Fulcio OIDC issuer extension
				Id:    asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1},
				Value: []byte(issuer),
			},
		},
	}

	if embedSCT {
		precert := createCertificate(t, template, ts.intermediateCert, ts.intermediateKey, &key.PublicKey)

		template.ExtraExtensions = append(template.ExtraExtensions, ts.signSCT(t, precert))
	}

	return createCertificate(t, template, ts.intermediateCert, ts.intermediateKey, &key.PublicKey), key
}

// signSCT returns the embedded SCT extension for the pre-certificate.
func (ts *testSigstore) signSCT(t *testing.T, precert *x509.Certificate) pkix.Extension {
	t.Helper()

	sct := ct.SignedCertificateTimestamp{
		SCTVersion: ct.V1,
		LogID:      ct.LogID{KeyID: ts.ctlogID},
		Timestamp:  uint64(time.Now().UnixMilli()), //nolint:gosec
	}

	input, err := ct.SerializeSCTSignatureInput(sct, ct.LogEntry{
		Leaf: ct.MerkleTreeLeaf{
			Version:  ct.V1,
			LeafType: ct.TimestampedEntryLeafType,
			TimestampedEntry: &ct.TimestampedEntry{
				Timestamp: sct.Timestamp,
				EntryType: ct.PrecertLogEntryType,
				PrecertEntry: &ct.PreCert{
					IssuerKeyHash:  sha256.Sum256(ts.intermediateCert.RawSubjectPublicKeyInfo),
					TBSCertificate: precert.RawTBSCertificate,
				},
			},
		},
	})
	require.NoError(t, err)

	hash := sha256.Sum256(input)

	sig, err := ecdsa.SignASN1(rand.Reader, ts.ctlogKey, hash[:])
	require.NoError(t, err)

	sct.Signature = ct.DigitallySigned{
		Algorithm: tls.SignatureAndHashAlgorithm{
			Hash:      tls.SHA256,
			Signature: tls.ECDSA,
		},
		Signature: sig,
	}

	sctList, err := x509util.MarshalSCTsIntoSCTList([]*ct.SignedCertificateTimestamp{&sct})
	require.NoError(t, err)

	sctBytes, err := tls.Marshal(*sctList)
	require.NoError(t, err)

	value, err := asn1.Marshal(sctBytes)
	require.NoError(t, err)

	return pkix.Extension{
		Id:    asn1.ObjectIdentifier(ctx509.OIDExtensionCTSCT),
		Value: value,
	}
}

// rekorBundle is the Rekor bundle as stored in the legacy signature layer annotation.
type rekorBundle struct {
	SignedEntryTimestamp []byte
	Payload              tlog.RekorPayload
}

// logEntry creates a Rekor hashedrekord entry for the signature and returns the bundle for it.
func (ts *testSigstore) logEntry(t *testing.T, signedPayload, sig []byte, cert *x509.Certificate) *rekorBundle {
	t.Helper()

	certPEM, err := cryptoutils.MarshalCertificateToPEM(cert)
	require.NoError(t, err)

	payloadDigest := sha256.Sum256(signedPayload)

	body, err := json.Marshal(map[string]any{
		"apiVersion": "0.0.1",
		"kind":       "hashedrekord",
		"spec": map[string]any{
			"data": map[string]any{
				"hash": map[string]any{
					"algorithm": "sha256",
					"value":     hex.EncodeToString(payloadDigest[:]),
				},
			},
			"signature": map[string]any{
				"content": base64.StdEncoding.EncodeToString(sig),
				"publicKey": map[string]any{
					"content": base64.StdEncoding.EncodeToString(certPEM),
				},
			},
		},
	})
	require.NoError(t, err)

	bundle := &rekorBundle{
		Payload: tlog.RekorPayload{
			Body:           base64.StdEncoding.EncodeToString(body),
			IntegratedTime: time.Now().Unix(),
			LogIndex:       1000,
			LogID:          hex.EncodeToString(ts.rekorLogID[:]),
		},
	}

	bundle.SignedEntryTimestamp = ts.signEntryTimestamp(t, bundle.Payload)

	return bundle
}

// signEntryTimestamp signs the Rekor payload (the inclusion promise).
func (ts *testSigstore) signEntryTimestamp(t *testing.T, rekorPayload tlog.RekorPayload) []byte {
	t.Helper()

	contents, err := json.Marshal(rekorPayload)
	require.NoError(t, err)

	canonicalized, err := jsoncanonicalizer.Transform(contents)
	require.NoError(t, err)

	hash := sha256.Sum256(canonicalized)

	set, err := ecdsa.SignASN1(rand.Reader, ts.rekorKey, hash[:])
	require.NoError(t, err)

	return set
}

// legacySignatureMaterial is the material of a legacy keyless cosign signature.
type legacySignatureMaterial struct {
	payload     []byte
	signature   []byte
	certificate *x509.Certificate
	bundle      *rekorBundle

	// annotationOverrides are applied on top of the generated annotations.
	annotationOverrides map[string]string
}

// sign signs the payload with a fresh certificate for the identity and logs the signature.
func (ts *testSigstore) sign(t *testing.T, signedPayload []byte, subject, issuer string) legacySignatureMaterial {
	t.Helper()

	cert, key := ts.issueCertificate(t, subject, issuer, true)

	return ts.signWith(t, signedPayload, cert, key)
}

// signWith signs the payload with the certificate key and logs the signature.
func (ts *testSigstore) signWith(t *testing.T, signedPayload []byte, cert *x509.Certificate, key *ecdsa.PrivateKey) legacySignatureMaterial {
	t.Helper()

	signer, err := signature.LoadECDSASignerVerifier(key, crypto.SHA256)
	require.NoError(t, err)

	sig, err := signer.SignMessage(bytes.NewReader(signedPayload))
	require.NoError(t, err)

	return legacySignatureMaterial{
		payload:     signedPayload,
		signature:   sig,
		certificate: cert,
		bundle:      ts.logEntry(t, signedPayload, sig, cert),
	}
}

// annotations returns the legacy signature layer annotations.
func (m *legacySignatureMaterial) annotations(t *testing.T, ts *testSigstore) map[string]string {
	t.Helper()

	certPEM, err := cryptoutils.MarshalCertificateToPEM(m.certificate)
	require.NoError(t, err)

	chainPEM, err := cryptoutils.MarshalCertificatesToPEM([]*x509.Certificate{ts.intermediateCert, ts.rootCert})
	require.NoError(t, err)

	annotations := map[string]string{
		signatureAnnotation:   base64.StdEncoding.EncodeToString(m.signature),
		certificateAnnotation: string(certPEM),
		chainAnnotation:       string(chainPEM),
	}

	if m.bundle != nil {
		bundleJSON, err := json.Marshal(m.bundle)
		require.NoError(t, err)

		annotations[bundleAnnotation] = string(bundleJSON)
	}

	maps.Copy(annotations, m.annotationOverrides)

	return annotations
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
}

func createCertificate(t *testing.T, template, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, pub *ecdsa.PublicKey) *x509.Certificate {
	t.Helper()

	der, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert
}

func newSerialNumber(t *testing.T) *big.Int {
	t.Helper()

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	return serial
}

// logID returns the transparency log ID: the SHA-256 of the DER-encoded public key.
func logID(t *testing.T, pub *ecdsa.PublicKey) [32]byte {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)

	return sha256.Sum256(der)
}

// TestVerifyLegacyKeyless verifies legacy keyless cosign signatures against a local Sigstore instance,
// tampering with each part of the signature material.
//
// The Rekor bundle annotation is converted to a sigstore-go transparency log entry by the package
// under test, so the test checks that every field of the bundle is authenticated.
func TestVerifyLegacyKeyless(t *testing.T) {
	t.Parallel()

	const (
		issuer  = "https://issuer.test"
		subject = "signer@example.com"
	)

	ts := newTestSigstore(t)
	repository := newLocalRegistry(t)
	resolver := newPlainHTTPResolver()

	for _, test := range []struct {
		name string

		// mutate tampers with the signature material before pushing it
		mutate func(t *testing.T, m *legacySignatureMaterial)

		expectedError string
	}{
		{
			name: "valid",
		},
		{
			name: "subject mismatch",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				*m = ts.sign(t, m.payload, "someone-else@example.com", issuer)
			},

			expectedError: "failed to verify certificate identity",
		},
		{
			name: "issuer mismatch",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				*m = ts.sign(t, m.payload, subject, "https://another-issuer.test")
			},

			expectedError: "failed to verify certificate identity",
		},
		{
			name: "tampered signed entry timestamp",
			mutate: func(_ *testing.T, m *legacySignatureMaterial) {
				m.bundle.SignedEntryTimestamp[len(m.bundle.SignedEntryTimestamp)-1] ^= 0xff
			},

			expectedError: "not enough verified log entries from transparency log: 0 < 1",
		},
		{
			name: "tampered integrated time",
			mutate: func(_ *testing.T, m *legacySignatureMaterial) {
				m.bundle.Payload.IntegratedTime++
			},

			expectedError: "not enough verified log entries from transparency log: 0 < 1",
		},
		{
			name: "tampered log index",
			mutate: func(_ *testing.T, m *legacySignatureMaterial) {
				m.bundle.Payload.LogIndex++
			},

			expectedError: "not enough verified log entries from transparency log: 0 < 1",
		},
		{
			name: "unknown transparency log",
			mutate: func(_ *testing.T, m *legacySignatureMaterial) {
				m.bundle.Payload.LogID = strings.Repeat("00", 32)
			},

			expectedError: "not enough verified log entries from transparency log: 0 < 1",
		},
		{
			name: "signature from another signing",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				m.signature = ts.sign(t, m.payload, subject, issuer).signature
			},

			expectedError: "transparency log signature does not match",
		},
		{
			name: "certificate from another signing",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				m.certificate = ts.sign(t, m.payload, subject, issuer).certificate
			},

			expectedError: "transparency log certificate does not match",
		},
		{
			name: "signature material for another payload",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				// consistent signature, certificate and log entry, but for a different payload
				other := ts.sign(t, []byte("another payload"), subject, issuer)

				m.signature, m.certificate, m.bundle = other.signature, other.certificate, other.bundle
			},

			expectedError: "transparency log hashedrekord entry digest",
		},
		{
			name: "integrated time outside certificate validity",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				m.bundle.Payload.IntegratedTime = m.certificate.NotAfter.Add(time.Hour).Unix()
				m.bundle.SignedEntryTimestamp = ts.signEntryTimestamp(t, m.bundle.Payload)
			},

			expectedError: "integrated time outside certificate validity",
		},
		{
			name: "certificate without SCT",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				cert, key := ts.issueCertificate(t, subject, issuer, false)

				*m = ts.signWith(t, m.payload, cert, key)
			},

			expectedError: "failed to verify signed certificate timestamp",
		},
		{
			name: "certificate from an untrusted CA",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				cert, key := newTestSigstore(t).issueCertificate(t, subject, issuer, true)

				*m = ts.signWith(t, m.payload, cert, key)
			},

			expectedError: "failed to verify leaf certificate",
		},
		{
			name: "signed payload for another image",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				*m = ts.sign(t, cosignPayload(t, pushRandomImage(t, repository)), subject, issuer)
			},

			expectedError: "invalid or missing digest in claim",
		},
		{
			name: "missing bundle annotation",
			mutate: func(_ *testing.T, m *legacySignatureMaterial) {
				m.bundle = nil
			},

			expectedError: "missing Rekor bundle annotation",
		},
		{
			name: "malformed bundle annotation",
			mutate: func(_ *testing.T, m *legacySignatureMaterial) {
				m.annotationOverrides = map[string]string{bundleAnnotation: "{"}
			},

			expectedError: "failed to parse Rekor bundle annotation",
		},
		{
			name: "malformed bundle body",
			mutate: func(t *testing.T, m *legacySignatureMaterial) {
				m.bundle.Payload.Body = base64.StdEncoding.EncodeToString([]byte(`{"kind":"hashedrekord","apiVersion":"0.0.1","spec":{}}`))
				m.bundle.SignedEntryTimestamp = ts.signEntryTimestamp(t, m.bundle.Payload)
			},

			expectedError: "failed to convert Rekor bundle",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			imageRef := pushRandomImage(t, repository)

			material := ts.sign(t, cosignPayload(t, imageRef), subject, issuer)

			if test.mutate != nil {
				test.mutate(t, &material)
			}

			pushLegacySignature(t, imageRef, material.payload, material.annotations(t, ts))

			checkOpts, err := ourcosign.NewKeylessCheckOpts(ts, issuer, subject, "")
			require.NoError(t, err)

			result, err := verifyLocalImage(t, resolver, imageRef, checkOpts)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, "no valid legacy signature")
				assert.ErrorContains(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "verified via legacy signature (bundle verified true)", result.Message)
		})
	}
}
