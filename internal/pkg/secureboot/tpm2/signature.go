// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// PCRData is the data structure for PCR signature json.
type PCRData struct {
	SHA1   []BankData `json:"sha1,omitempty"`
	SHA256 []BankData `json:"sha256,omitempty"`
	SHA384 []BankData `json:"sha384,omitempty"`
	SHA512 []BankData `json:"sha512,omitempty"`
}

// BankData constains data for a specific PCR bank.
type BankData struct {
	// list of PCR banks
	PCRs []int `json:"pcrs"`
	// Public key of the TPM
	PKFP string `json:"pkfp"`
	// Policy digest
	Pol string `json:"pol"`
	// Signature of the policy digest in base64
	Sig string `json:"sig"`
}

// ParsePCRSignature parses the PCR signature json file.
func ParsePCRSignature() (*PCRData, error) {
	pcrSignature, err := os.ReadFile(constants.PCRSignatureJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to read pcr signature: %v", err)
	}

	pcrData := &PCRData{}

	if err = json.Unmarshal(pcrSignature, pcrData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pcr signature: %v", err)
	}

	return pcrData, nil
}
