// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

const (
	// EncryptionSchemaVersionErrata is the errata for the encryption schema version.
	// Talos versions older than 1.12 locked to PCR 7 and PCR 11 but the luks json header only
	// saved the PCR 11 value, so if the version is not set or empty we can assume that the keys
	// are sealed to both PCR 7 and PCR 11. If the version is `1` we can be sure that the keys
	// are locked to PCR 11 only.
	EncryptionSchemaVersionErrata = "1"
)

// SealedResponse is the response from the TPM2.0 Seal operation.
type SealedResponse struct {
	SealedBlobPrivate []byte
	SealedBlobPublic  []byte
	KeyName           []byte
	PolicyDigest      []byte
	PCRs              []int
	PubKeyPCRs        []int
	EncryptionVersion string
	Alg               string
}
