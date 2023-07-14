// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

import (
	"crypto/sha256"
	"fmt"

	"github.com/google/go-tpm/tpm2"
)

// CalculatePolicy calculates the policy hash for a given PCR value and PCR selection.
func CalculatePolicy(pcrValue []byte, pcrSelection tpm2.TPMLPCRSelection) ([]byte, error) {
	calculator, err := tpm2.NewPolicyCalculator(tpm2.TPMAlgSHA256)
	if err != nil {
		return nil, err
	}

	pcrHash := sha256.Sum256(pcrValue)

	policy := tpm2.PolicyPCR{
		PcrDigest: tpm2.TPM2BDigest{
			Buffer: pcrHash[:],
		},
		Pcrs: pcrSelection,
	}

	if err := policy.Update(calculator); err != nil {
		return nil, err
	}

	return calculator.Hash().Digest, nil
}

// CalculateSealingPolicyDigest calculates the sealing policy digest for a given PCR value, PCR selection and public key.
func CalculateSealingPolicyDigest(pcrValue []byte, pcrSelection tpm2.TPMLPCRSelection, pubKey string) ([]byte, error) {
	calculator, err := tpm2.NewPolicyCalculator(tpm2.TPMAlgSHA256)
	if err != nil {
		return nil, err
	}

	pubKeyData, err := ParsePCRSigningPubKey(pubKey)
	if err != nil {
		return nil, err
	}

	publicKeyTemplate := RSAPubKeyTemplate(pubKeyData.N.BitLen(), pubKeyData.E, pubKeyData.N.Bytes())

	name, err := tpm2.ObjectName(&publicKeyTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate name: %v", err)
	}

	policyAuthorize := tpm2.PolicyAuthorize{
		KeySign: *name,
	}

	if err := policyAuthorize.Update(calculator); err != nil {
		return nil, err
	}

	pcrHash := sha256.Sum256(pcrValue)

	policy := tpm2.PolicyPCR{
		PcrDigest: tpm2.TPM2BDigest{
			Buffer: pcrHash[:],
		},
		Pcrs: pcrSelection,
	}

	if err := policy.Update(calculator); err != nil {
		return nil, err
	}

	return calculator.Hash().Digest, nil
}
