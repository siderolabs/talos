// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

import (
	"crypto/sha256"
	"fmt"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
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

// calculatePolicyAuthorize creates and updates a PolicyAuthorize for the given public key.
func calculatePolicyAuthorize(calculator *tpm2.PolicyCalculator, pubKey string) error {
	pubKeyData, err := ParsePCRSigningPubKey(pubKey)
	if err != nil {
		return err
	}

	publicKeyTemplate := RSAPubKeyTemplate(pubKeyData.N.BitLen(), pubKeyData.E, pubKeyData.N.Bytes())

	name, err := tpm2.ObjectName(&publicKeyTemplate)
	if err != nil {
		return fmt.Errorf("failed to calculate name: %v", err)
	}

	policyAuthorize := tpm2.PolicyAuthorize{
		KeySign: *name,
	}

	return policyAuthorize.Update(calculator)
}

// SealingPolicyDigestInfo holds the information needed to calculate a sealing policy digest.
type SealingPolicyDigestInfo struct {
	PublicKey   string
	PCRs        []int
	ReadPCRFunc func(t transport.TPM, pcr int) ([]byte, error)
}

// CalculateSealingPolicyDigest calculates the sealing policy digest for a given public key and PCRs.
func CalculateSealingPolicyDigest(t transport.TPM, spInfo SealingPolicyDigestInfo) ([]byte, error) {
	calculator, err := tpm2.NewPolicyCalculator(tpm2.TPMAlgSHA256)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy calculator: %v", err)
	}

	if err := calculatePolicyAuthorize(calculator, spInfo.PublicKey); err != nil {
		return nil, fmt.Errorf("failed to calculate policy authorize: %v", err)
	}

	if len(spInfo.PCRs) == 0 {
		return calculator.Hash().Digest, nil
	}

	pcrSelector, err := CreateSelector(spInfo.PCRs)
	if err != nil {
		return nil, fmt.Errorf("failed to create PCR selection for PCRs %v: %v", spInfo.PCRs, err)
	}

	pcrSelection := tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      tpm2.TPMAlgSHA256,
				PCRSelect: pcrSelector,
			},
		},
	}

	hash := sha256.New()

	for _, p := range spInfo.PCRs {
		pcrValue, err := spInfo.ReadPCRFunc(t, p)
		if err != nil {
			return nil, fmt.Errorf("failed to read PCR %d: %v", p, err)
		}

		if _, err := hash.Write(pcrValue); err != nil {
			return nil, fmt.Errorf("failed to hash PCR value for PCR %d: %v", p, err)
		}
	}

	policy := tpm2.PolicyPCR{
		PcrDigest: tpm2.TPM2BDigest{
			Buffer: hash.Sum(nil),
		},
		Pcrs: pcrSelection,
	}

	if err := policy.Update(calculator); err != nil {
		return nil, fmt.Errorf("failed to update policy digest for PCRs %v: %v", spInfo.PCRs, err)
	}

	return calculator.Hash().Digest, nil
}
