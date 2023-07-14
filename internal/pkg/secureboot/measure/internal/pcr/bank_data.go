// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pcr

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/google/go-tpm/tpm2"

	"github.com/siderolabs/talos/internal/pkg/secureboot"
	talostpm2 "github.com/siderolabs/talos/internal/pkg/tpm2"
)

// CalculateBankData calculates the PCR bank data for a given set of UKI file sections.
//
// This mimics the process happening happening in the TPM when the UKI is being loaded.
func CalculateBankData(pcrNumber int, alg tpm2.TPMAlgID, sectionData map[secureboot.Section]string, rsaKey *rsa.PrivateKey) ([]talostpm2.BankData, error) {
	// get fingerprint of public key
	pubKeyFingerprint := sha256.Sum256(x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey))

	hashAlg, err := alg.Hash()
	if err != nil {
		return nil, err
	}

	pcrSelector, err := CreateSelector([]int{secureboot.UKIPCR})
	if err != nil {
		return nil, fmt.Errorf("failed to create PCR selection: %v", err)
	}

	pcrSelection := tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      alg,
				PCRSelect: pcrSelector,
			},
		},
	}

	hashData := NewDigest(hashAlg)

	for _, section := range secureboot.OrderedSections() {
		if file := sectionData[section]; file != "" {
			hashData.Extend(append([]byte(section), 0))

			sectionData, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}

			hashData.Extend(sectionData)
		}
	}

	banks := make([]talostpm2.BankData, 0)

	for _, phaseInfo := range secureboot.OrderedPhases() {
		// extend always, but only calculate signature if requested
		hashData.Extend([]byte(phaseInfo.Phase))

		if !phaseInfo.CalculateSignature {
			continue
		}

		hash := hashData.Hash()

		policyPCR, err := CalculatePolicy(hash, pcrSelection)
		if err != nil {
			return nil, err
		}

		sigData, err := Sign(policyPCR, hashAlg, rsaKey)
		if err != nil {
			return nil, err
		}

		banks = append(banks, talostpm2.BankData{
			PCRs: []int{pcrNumber},
			PKFP: hex.EncodeToString(pubKeyFingerprint[:]),
			Sig:  sigData.SignatureBase64,
			Pol:  sigData.Digest,
		})
	}

	return banks, nil
}
