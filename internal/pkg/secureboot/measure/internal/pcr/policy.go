// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pcr

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/go-tpm/tpm2"
)

// CalculatePolicy calculates the policy hash for a given PCR value and PCR selection.
func CalculatePolicy(pcrValue []byte, pcrSelection tpm2.TPMLPCRSelection) []byte {
	initial := make([]byte, sha256.Size)
	pcrHash := sha256.Sum256(pcrValue)

	policyPCRCommandValue := make([]byte, 4)
	binary.BigEndian.PutUint32(policyPCRCommandValue, uint32(tpm2.TPMCCPolicyPCR))

	pcrSelectionMarshalled := tpm2.Marshal(pcrSelection)

	hasher := sha256.New()
	hasher.Write(initial)
	hasher.Write(policyPCRCommandValue)
	hasher.Write(pcrSelectionMarshalled)
	hasher.Write(pcrHash[:])

	return hasher.Sum(nil)
}
