// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

const (
	// SecureBootStatePCR is the PCR number where the secure boot state and the signature are measured.
	// PCR 7 changes when UEFI SecureBoot mode is enabled/disabled, or firmware certificates (PK, KEK, db, dbx, …) are updated.
	SecureBootStatePCR = 7
)

// SealedResponse is the response from the TPM2.0 Seal operation.
type SealedResponse struct {
	SealedBlobPrivate []byte
	SealedBlobPublic  []byte
	KeyName           []byte
	PolicyDigest      []byte
}
