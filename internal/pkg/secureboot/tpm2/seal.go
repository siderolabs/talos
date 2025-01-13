// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

import (
	"fmt"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"

	"github.com/siderolabs/talos/internal/pkg/tpm"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Seal seals the key using TPM2.0.
func Seal(key []byte) (*SealedResponse, error) {
	t, err := tpm.Open()
	if err != nil {
		return nil, err
	}
	defer t.Close() //nolint:errcheck

	// fail early if PCR banks are not present or filled with all zeroes or 0xff
	if err = validatePCRBanks(t); err != nil {
		return nil, err
	}

	sealingPolicyDigest, err := calculateSealingPolicyDigest(t)
	if err != nil {
		return nil, err
	}

	primary := tpm2.CreatePrimary{
		PrimaryHandle: tpm2.TPMRHOwner,
		InPublic:      tpm2.New2B(tpm2.ECCSRKTemplate),
	}

	createPrimaryResponse, err := primary.Execute(t)
	if err != nil {
		return nil, err
	}

	defer func() {
		flush := tpm2.FlushContext{
			FlushHandle: createPrimaryResponse.ObjectHandle,
		}

		_, flushErr := flush.Execute(t)
		if flushErr != nil {
			err = flushErr
		}
	}()

	outPub, err := createPrimaryResponse.OutPublic.Contents()
	if err != nil {
		return nil, err
	}

	create := tpm2.Create{
		ParentHandle: tpm2.AuthHandle{
			Handle: createPrimaryResponse.ObjectHandle,
			Name:   createPrimaryResponse.Name,
			Auth: tpm2.HMAC(
				tpm2.TPMAlgSHA256,
				20,
				tpm2.Salted(createPrimaryResponse.ObjectHandle, *outPub),
				tpm2.AESEncryption(128, tpm2.EncryptInOut),
			),
		},
		InSensitive: tpm2.TPM2BSensitiveCreate{
			Sensitive: &tpm2.TPMSSensitiveCreate{
				Data: tpm2.NewTPMUSensitiveCreate(&tpm2.TPM2BSensitiveData{
					Buffer: key,
				}),
			},
		},
		InPublic: tpm2.New2B(tpm2.TPMTPublic{
			Type:    tpm2.TPMAlgKeyedHash,
			NameAlg: tpm2.TPMAlgSHA256,
			ObjectAttributes: tpm2.TPMAObject{
				FixedTPM:    true,
				FixedParent: true,
			},
			Parameters: tpm2.NewTPMUPublicParms(tpm2.TPMAlgKeyedHash, &tpm2.TPMSKeyedHashParms{
				Scheme: tpm2.TPMTKeyedHashScheme{
					Scheme: tpm2.TPMAlgNull,
				},
			}),
			AuthPolicy: tpm2.TPM2BDigest{
				Buffer: sealingPolicyDigest,
			},
		}),
	}

	createResp, err := create.Execute(t)
	if err != nil {
		return nil, err
	}

	resp := SealedResponse{
		SealedBlobPrivate: tpm2.Marshal(createResp.OutPrivate),
		SealedBlobPublic:  tpm2.Marshal(createResp.OutPublic),
		KeyName:           tpm2.Marshal(createPrimaryResponse.Name),
		PolicyDigest:      sealingPolicyDigest,
	}

	return &resp, nil
}

func calculateSealingPolicyDigest(t transport.TPM) ([]byte, error) {
	pcrSelector, err := CreateSelector([]int{SecureBootStatePCR})
	if err != nil {
		return nil, fmt.Errorf("failed to create PCR selection: %v", err)
	}

	pcrSelection := tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      tpm2.TPMAlgSHA256,
				PCRSelect: pcrSelector,
			},
		},
	}

	pcrValue, err := ReadPCR(t, SecureBootStatePCR)
	if err != nil {
		return nil, err
	}

	sealingDigest, err := CalculateSealingPolicyDigest(pcrValue, pcrSelection, constants.PCRPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate sealing policy digest: %v", err)
	}

	return sealingDigest, nil
}
