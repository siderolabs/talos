// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"log"
	"os"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"

	"github.com/siderolabs/talos/internal/pkg/tpm"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// CreateSelector converts PCR  numbers into a bitmask.
func CreateSelector(pcrs []int) ([]byte, error) {
	const sizeOfPCRSelect = 3

	mask := make([]byte, sizeOfPCRSelect)

	for _, n := range pcrs {
		if n >= 8*sizeOfPCRSelect {
			return nil, fmt.Errorf("PCR index %d is out of range (exceeds maximum value %d)", n, 8*sizeOfPCRSelect-1)
		}

		mask[n>>3] |= 1 << (n & 0x7)
	}

	return mask, nil
}

// ReadPCR reads the value of a single PCR.
func ReadPCR(t transport.TPM, pcr int) ([]byte, error) {
	pcrSelector, err := CreateSelector([]int{pcr})
	if err != nil {
		return nil, fmt.Errorf("failed to create PCR selection: %w", err)
	}

	pcrRead := tpm2.PCRRead{
		PCRSelectionIn: tpm2.TPMLPCRSelection{
			PCRSelections: []tpm2.TPMSPCRSelection{
				{
					Hash:      tpm2.TPMAlgSHA256,
					PCRSelect: pcrSelector,
				},
			},
		},
	}

	pcrValue, err := pcrRead.Execute(t)
	if err != nil {
		return nil, fmt.Errorf("failed to read PCR: %w", err)
	}

	return pcrValue.PCRValues.Digests[0].Buffer, nil
}

// PCRExtend hashes the input and extends the PCR with the hash.
func PCRExtend(pcr int, data []byte) error {
	t, err := tpm.Open()
	if err != nil {
		// if the TPM is not available we can skip the PCR extension
		if os.IsNotExist(err) {
			log.Printf("TPM device is not available, skipping PCR extension")

			return nil
		}

		return err
	}

	defer t.Close() //nolint:errcheck

	// now we need to check if the TPM is a 2.0 device
	// we can do this by checking the manufacturer,
	// if it fails, we can skip the PCR extension
	_, err = tpm2.GetCapability{
		Capability:    tpm2.TPMCapTPMProperties,
		Property:      uint32(tpm2.TPMPTManufacturer),
		PropertyCount: 1,
	}.Execute(t)
	if err != nil {
		log.Printf("TPM device is not a TPM 2.0, skipping PCR extension")

		return nil
	}

	// since we are using SHA256, we can assume that the PCR bank is SHA256
	digest := sha256.Sum256(data)

	pcrHandle := tpm2.PCRExtend{
		PCRHandle: tpm2.AuthHandle{
			Handle: tpm2.TPMHandle(pcr),
			Auth:   tpm2.PasswordAuth(nil),
		},
		Digests: tpm2.TPMLDigestValues{
			Digests: []tpm2.TPMTHA{
				{
					HashAlg: tpm2.TPMAlgSHA256,
					Digest:  digest[:],
				},
			},
		},
	}

	if _, err = pcrHandle.Execute(t); err != nil {
		return err
	}

	return nil
}

// PolicyPCRDigest executes policyPCR and returns the digest.
func PolicyPCRDigest(t transport.TPM, policyHandle tpm2.TPMHandle, pcrSelection tpm2.TPMLPCRSelection) (*tpm2.TPM2BDigest, error) {
	policyPCR := tpm2.PolicyPCR{
		PolicySession: policyHandle,
		Pcrs:          pcrSelection,
	}

	if _, err := policyPCR.Execute(t); err != nil {
		return nil, fmt.Errorf("failed to execute policyPCR: %w", err)
	}

	policyGetDigest := tpm2.PolicyGetDigest{
		PolicySession: policyHandle,
	}

	policyGetDigestResponse, err := policyGetDigest.Execute(t)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy digest: %w", err)
	}

	return &policyGetDigestResponse.PolicyDigest, nil
}

//nolint:gocyclo
func validatePCRBanks(t transport.TPM) error {
	pcrValue, err := ReadPCR(t, constants.UKIPCR)
	if err != nil {
		return fmt.Errorf("failed to read PCR: %w", err)
	}

	if err = validatePCRNotZeroAndNotFilled(pcrValue, constants.UKIPCR); err != nil {
		return err
	}

	pcrValue, err = ReadPCR(t, SecureBootStatePCR)
	if err != nil {
		return fmt.Errorf("failed to read PCR: %w", err)
	}

	if err = validatePCRNotZeroAndNotFilled(pcrValue, SecureBootStatePCR); err != nil {
		return err
	}

	caps := tpm2.GetCapability{
		Capability:    tpm2.TPMCapPCRs,
		Property:      0,
		PropertyCount: 1,
	}

	capsResp, err := caps.Execute(t)
	if err != nil {
		return fmt.Errorf("failed to get PCR capabilities: %w", err)
	}

	assignedPCRs, err := capsResp.CapabilityData.Data.AssignedPCR()
	if err != nil {
		return fmt.Errorf("failed to parse assigned PCRs: %w", err)
	}

	for _, s := range assignedPCRs.PCRSelections {
		if s.Hash != tpm2.TPMAlgSHA256 {
			continue
		}

		// check if 24 banks are available
		if len(s.PCRSelect) != 24/8 {
			return fmt.Errorf("unexpected number of PCR banks: %d", len(s.PCRSelect))
		}

		// check if all banks are available
		if s.PCRSelect[0] != 0xff || s.PCRSelect[1] != 0xff || s.PCRSelect[2] != 0xff {
			return fmt.Errorf("unexpected PCR banks: %v", s.PCRSelect)
		}
	}

	return nil
}

func validatePCRNotZeroAndNotFilled(pcrValue []byte, pcr int) error {
	if bytes.Equal(pcrValue, bytes.Repeat([]byte{0x00}, sha256.Size)) {
		return fmt.Errorf("PCR bank %d is populated with all zeroes", pcr)
	}

	if bytes.Equal(pcrValue, bytes.Repeat([]byte{0xFF}, sha256.Size)) {
		return fmt.Errorf("PCR bank %d is populated with all 0xFF", pcr)
	}

	return nil
}
