// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/go-tpm/tpm2"

	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/internal/pkg/tpm"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Unseal unseals a sealed blob using the TPM
//
//nolint:gocyclo,cyclop
func Unseal(sealed SealedResponse) ([]byte, error) {
	t, err := tpm.Open()
	if err != nil {
		return nil, err
	}
	defer t.Close() //nolint:errcheck

	// fail early if PCR banks are not present or filled with all zeroes or 0xff
	if err = validatePCRBanks(t); err != nil {
		return nil, err
	}

	tpmPub, err := tpm2.Unmarshal[tpm2.TPM2BPublic](sealed.SealedBlobPublic)
	if err != nil {
		return nil, err
	}

	tpmPriv, err := tpm2.Unmarshal[tpm2.TPM2BPrivate](sealed.SealedBlobPrivate)
	if err != nil {
		return nil, err
	}

	srk, err := tpm2.Unmarshal[tpm2.TPM2BName](sealed.KeyName)
	if err != nil {
		return nil, err
	}

	// we need to create a primary since we don't persist the SRK
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

	if !bytes.Equal(createPrimaryResponse.Name.Buffer, srk.Buffer) {
		// this means the srk name does not match, possibly due to a different TPM or tpm was reset
		// could also mean the disk was used on a different machine
		return nil, errors.New("srk name does not match")
	}

	load := tpm2.Load{
		ParentHandle: tpm2.NamedHandle{
			Handle: createPrimaryResponse.ObjectHandle,
			Name:   createPrimaryResponse.Name,
		},
		InPrivate: *tpmPriv,
		InPublic:  *tpmPub,
	}

	loadResponse, err := load.Execute(t)
	if err != nil {
		return nil, err
	}

	policySess, policyCloseFunc, err := tpm2.PolicySession(
		t,
		tpm2.TPMAlgSHA256,
		20,
		tpm2.Salted(createPrimaryResponse.ObjectHandle, *outPub),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy session: %w", err)
	}

	defer policyCloseFunc() //nolint:errcheck

	pubKey, err := ParsePCRSigningPubKey(constants.PCRPublicKey)
	if err != nil {
		return nil, err
	}

	loadExternal := tpm2.LoadExternal{
		Hierarchy: tpm2.TPMRHOwner,
		InPublic:  tpm2.New2B(RSAPubKeyTemplate(pubKey.N.BitLen(), pubKey.E, pubKey.N.Bytes())),
	}

	loadExternalResponse, err := loadExternal.Execute(t)
	if err != nil {
		return nil, fmt.Errorf("failed to load external key: %w", err)
	}

	defer func() {
		flush := tpm2.FlushContext{
			FlushHandle: loadExternalResponse.ObjectHandle,
		}

		_, flushErr := flush.Execute(t)
		if flushErr != nil {
			err = flushErr
		}
	}()

	pcrSelector, err := CreateSelector([]int{secureboot.UKIPCR})
	if err != nil {
		return nil, err
	}

	policyDigest, err := PolicyPCRDigest(t, policySess.Handle(), tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      tpm2.TPMAlgSHA256,
				PCRSelect: pcrSelector,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve policy digest: %w", err)
	}

	sigJSON, err := ParsePCRSignature()
	if err != nil {
		return nil, err
	}

	pubKeyFingerprint := sha256.Sum256(x509.MarshalPKCS1PublicKey(pubKey))

	var signature string
	// TODO: maybe we should use the highest supported algorithm of the TPM
	// fallback to the next one if the signature is not found
	for _, bank := range sigJSON.SHA256 {
		digest, decodeErr := hex.DecodeString(bank.Pol)
		if decodeErr != nil {
			return nil, decodeErr
		}

		if bytes.Equal(digest, policyDigest.Buffer) {
			signature = bank.Sig

			if hex.EncodeToString(pubKeyFingerprint[:]) != bank.PKFP {
				return nil, errors.New("certificate fingerprint does not match")
			}

			break
		}
	}

	if signature == "" {
		return nil, errors.New("signature not found")
	}

	signatureDecoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, err
	}

	// Verify will only verify the RSA part of the RSA+SHA256 signature,
	// hence we need to do the SHA256 part ourselves
	policyDigestHash := sha256.Sum256(policyDigest.Buffer)

	verifySignature := tpm2.VerifySignature{
		KeyHandle: loadExternalResponse.ObjectHandle,
		Digest: tpm2.TPM2BDigest{
			Buffer: policyDigestHash[:],
		},
		Signature: tpm2.TPMTSignature{
			SigAlg: tpm2.TPMAlgRSASSA,
			Signature: tpm2.NewTPMUSignature(tpm2.TPMAlgRSASSA, &tpm2.TPMSSignatureRSA{
				Hash: tpm2.TPMAlgSHA256,
				Sig: tpm2.TPM2BPublicKeyRSA{
					Buffer: signatureDecoded,
				},
			}),
		},
	}

	verifySignatureResponse, err := verifySignature.Execute(t)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	policyAuthorize := tpm2.PolicyAuthorize{
		PolicySession:  policySess.Handle(),
		ApprovedPolicy: *policyDigest,
		KeySign:        loadExternalResponse.Name,
		CheckTicket:    verifySignatureResponse.Validation,
	}

	if _, err = policyAuthorize.Execute(t); err != nil {
		return nil, fmt.Errorf("failed to execute policy authorize: %w", err)
	}

	secureBootStatePCRSelector, err := CreateSelector([]int{secureboot.SecureBootStatePCR})
	if err != nil {
		return nil, err
	}

	secureBootStatePolicyDigest, err := PolicyPCRDigest(t, policySess.Handle(), tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      tpm2.TPMAlgSHA256,
				PCRSelect: secureBootStatePCRSelector,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to calculate policy PCR digest: %w", err)
	}

	if !bytes.Equal(secureBootStatePolicyDigest.Buffer, sealed.PolicyDigest) {
		return nil, errors.New("sealing policy digest does not match")
	}

	unsealOp := tpm2.Unseal{
		ItemHandle: tpm2.AuthHandle{
			Handle: loadResponse.ObjectHandle,
			Name:   loadResponse.Name,
			Auth:   policySess,
		},
	}

	unsealResponse, err := unsealOp.Execute(t, tpm2.HMAC(
		tpm2.TPMAlgSHA256,
		20,
		tpm2.Salted(createPrimaryResponse.ObjectHandle, *outPub),
		tpm2.AESEncryption(128, tpm2.EncryptOut),
		tpm2.Bound(loadResponse.ObjectHandle, loadResponse.Name, nil),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to unseal op: %w", err)
	}

	return unsealResponse.OutData.Buffer, nil
}
