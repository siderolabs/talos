// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventlog_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"math/big"
	"slices"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/secureboot/eventlog"
)

// TCG event types which appear in PCR 7.
const (
	evNoAction                = 0x00000003
	evSeparator               = 0x00000004
	evEFIVariableDriverConfig = 0x80000001
	evEFIVariableAuthority    = 0x800000E0
)

const (
	// algSHA256 is the TPM_ALG_ID of SHA-256.
	algSHA256 = 0x000B
	// sha1DigestSize is the digest size of the legacy event log header entry.
	sha1DigestSize = 20
	// signatureListHeaderSize is the size of an EFI_SIGNATURE_LIST header.
	signatureListHeaderSize = 28
	// guidSize is the size of an EFI_GUID.
	guidSize = 16
)

// efiCertX509GUID is EFI_CERT_X509_GUID, marking a signature list which holds DER certificates.
var efiCertX509GUID = [guidSize]byte{
	0xa1, 0x59, 0xc0, 0xa5, 0xe4, 0x94, 0xa7, 0x4a,
	0x87, 0xb5, 0xab, 0x15, 0x5c, 0x2b, 0xf0, 0x72,
}

// testOwnerGUID stands in for the EFI_SIGNATURE_DATA signature owner, which is not interpreted.
var testOwnerGUID = [guidSize]byte{
	0xbd, 0x9a, 0xfa, 0x77, 0x59, 0x03, 0x32, 0x4d,
	0xbd, 0x60, 0x28, 0xf4, 0xe7, 0x8f, 0x78, 0x4b,
}

func le16(v uint16) []byte {
	return binary.LittleEndian.AppendUint16(nil, v)
}

func le32(v uint32) []byte {
	return binary.LittleEndian.AppendUint32(nil, v)
}

func le64(v uint64) []byte {
	return binary.LittleEndian.AppendUint64(nil, v)
}

// selfSignedCert generates a certificate to stand in for a SecureBoot `db` entry.
func selfSignedCert(t *testing.T, commonName string) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	return der
}

// signatureList encodes a single-entry EFI_SIGNATURE_LIST of X.509 certificates, as measured
// for the `PK`, `KEK` and `db` variables.
func signatureList(der []byte) []byte {
	return slices.Concat(
		efiCertX509GUID[:],
		le32(uint32(signatureListHeaderSize+guidSize+len(der))), // SignatureListSize
		le32(0),                         // SignatureHeaderSize
		le32(uint32(guidSize+len(der))), // SignatureSize
		testOwnerGUID[:],
		der,
	)
}

// signatureData encodes the EFI_SIGNATURE_DATA which an EV_EFI_VARIABLE_AUTHORITY event carries.
func signatureData(der []byte) []byte {
	return slices.Concat(testOwnerGUID[:], der)
}

// variableData encodes a UEFI_VARIABLE_DATA event body.
func variableData(name string, data []byte) []byte {
	utf16Name := utf16.Encode([]rune(name))

	out := slices.Concat(
		make([]byte, guidSize), // VariableName GUID, not interpreted for the variables used here
		le64(uint64(len(utf16Name))),
		le64(uint64(len(data))),
	)

	for _, char := range utf16Name {
		out = append(out, le16(char)...)
	}

	return append(out, data...)
}

// specIDEvent builds the TCG_EfiSpecIdEventStruct which switches the log over to the
// crypto-agile format with a single SHA-256 bank.
func specIDEvent() []byte {
	return slices.Concat(
		[]byte("Spec ID Event03\x00"),
		le32(0),            // PlatformClass
		[]byte{0, 2, 0, 8}, // VersionMinor, VersionMajor, Errata, UintnSize
		le32(1),            // NumAlgs
		le16(algSHA256),
		le16(sha256.Size),
		[]byte{0}, // VendorInfoSize
	)
}

type event struct {
	data []byte
	typ  uint32
}

// buildLog assembles a crypto-agile TCG event log out of PCR 7 events, and returns it together
// with the PCR 7 value those events replay to.
func buildLog(events []event) (rawLog, pcr7 []byte) {
	header := specIDEvent()

	// the first entry is in the legacy SHA-1 format and is not extended into any PCR
	rawLog = slices.Concat(
		le32(0), // PCRIndex
		le32(evNoAction),
		make([]byte, sha1DigestSize),
		le32(uint32(len(header))),
		header,
	)

	pcr7 = make([]byte, sha256.Size)

	for _, ev := range events {
		digest := sha256.Sum256(ev.data)

		rawLog = slices.Concat(
			rawLog,
			le32(eventlog.SecureBootPCR),
			le32(ev.typ),
			le32(1), // digest count
			le16(algSHA256),
			digest[:],
			le32(uint32(len(ev.data))),
			ev.data,
		)

		extended := sha256.Sum256(slices.Concat(pcr7, digest[:]))
		pcr7 = extended[:]
	}

	return rawLog, pcr7
}

// secureBootEvents builds the PCR 7 events of a machine which booted with SecureBoot enabled
// and had its image authorized by the given `db` certificate.
func secureBootEvents(t *testing.T, db []byte) []event {
	t.Helper()

	return []event{
		{variableData("SecureBoot", []byte{1}), evEFIVariableDriverConfig},
		{variableData("PK", signatureList(selfSignedCert(t, "test PK"))), evEFIVariableDriverConfig},
		{variableData("KEK", signatureList(selfSignedCert(t, "test KEK"))), evEFIVariableDriverConfig},
		{variableData("db", signatureList(db)), evEFIVariableDriverConfig},
		{[]byte{0, 0, 0, 0}, evSeparator},
		{variableData("db", signatureData(db)), evEFIVariableAuthority},
	}
}

func TestParseSecureBootState(t *testing.T) {
	t.Parallel()

	db := selfSignedCert(t, "test db")

	rawLog, pcr7 := buildLog(secureBootEvents(t, db))

	state, err := eventlog.ParseSecureBootState(rawLog, pcr7)
	require.NoError(t, err)

	assert.True(t, state.Enabled)
	assert.Empty(t, state.PreSeparatorAuthority)

	require.Len(t, state.PostSeparatorAuthority, 1)
	assert.Equal(t, "test db", state.PostSeparatorAuthority[0].Subject.CommonName)
}

// TestParseSecureBootStateWrongPCR asserts that a log which does not replay to the PCR value
// read back from the TPM is rejected rather than believed.
func TestParseSecureBootStateWrongPCR(t *testing.T) {
	t.Parallel()

	rawLog, pcr7 := buildLog(secureBootEvents(t, selfSignedCert(t, "test db")))

	pcr7[0] ^= 0xff

	_, err := eventlog.ParseSecureBootState(rawLog, pcr7)
	assert.ErrorContains(t, err, "failed to replay PCR 7")
}

// TestParseSecureBootStateTamperedEvent asserts that an event whose data no longer matches the
// digest measured for it is rejected, even though the log as a whole still replays to PCR 7.
func TestParseSecureBootStateTamperedEvent(t *testing.T) {
	t.Parallel()

	rawLog, pcr7 := buildLog(secureBootEvents(t, selfSignedCert(t, "test db")))

	// the authority event is last in the log and its data is last within the event, so the
	// final byte belongs to the signature of the certificate the firmware trusted: flipping it
	// leaves the log replaying to the same PCR 7, because the recorded digests are untouched
	tampered := slices.Clone(rawLog)
	tampered[len(tampered)-1] ^= 0xff

	_, err := eventlog.ParseSecureBootState(tampered, pcr7)
	assert.ErrorContains(t, err, "invalid digest for authority")
}
