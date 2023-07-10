// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package measure

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"

	"github.com/siderolabs/ukify/constants"
	"github.com/siderolabs/ukify/measure/extend"
)

type PCRData struct {
	SHA1   []bankData `json:"sha1,omitempty"`
	SHA256 []bankData `json:"sha256,omitempty"`
	SHA384 []bankData `json:"sha384,omitempty"`
	SHA512 []bankData `json:"sha512,omitempty"`
}

type bankData struct {
	// list of PCR banks
	PCRS []int `json:"pcrs"`
	// Public key of the TPM
	PKFP string `json:"pkfp"`
	// Policy digest
	POL string `json:"pol"`
	// Signature of the policy digest in base64
	SIG string `json:"sig"`
}

// signatureData returns the hashed signature digest and base64 encoded signature
type signatureData struct {
	Digest          string
	SignatureBase64 string
}

// SectionData holds a map of Section to file path to the corresponding section
type SectionsData map[constants.Section]string

func calculatePCRBankData(pcr int, alg tpm2.TPMAlgID, sectionData SectionsData, privateKeyFile string) ([]bankData, error) {
	rsaKey, err := parseRSAKey(privateKeyFile)
	if err != nil {
		return nil, err
	}

	// get fingerprint of public key
	pubKeyFingerprint := sha256.Sum256(x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey))

	hashAlg, err := alg.Hash()
	if err != nil {
		return nil, err
	}

	pcrSelector, err := createPCRSelection([]int{constants.UKIPCR})
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

	hashData := extend.New(hashAlg)

	for _, section := range constants.OrderedSections() {
		if file, ok := sectionData[section]; ok && file != "" {
			hashData.Extend(append([]byte(section), 0))

			sectionData, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}

			hashData.Extend(sectionData)
		}
	}

	banks := make([]bankData, len(constants.OrderedPhases()))

	for i, phase := range constants.OrderedPhases() {
		hashData.Extend([]byte(phase))

		hash := hashData.Hash()

		policyPCR := calculatePolicyPCR(hash, pcrSelection)

		sigData, err := calculateSignature(policyPCR, hashAlg, rsaKey)
		if err != nil {
			return nil, err
		}

		banks[i] = bankData{
			PCRS: []int{pcr},
			PKFP: hex.EncodeToString(pubKeyFingerprint[:]),
			SIG:  sigData.SignatureBase64,
			POL:  sigData.Digest,
		}
	}

	return banks, nil
}

func calculatePolicyPCR(pcrValue []byte, pcrSelection tpm2.TPMLPCRSelection) []byte {
	initial := bytes.Repeat([]byte{0x00}, sha256.Size)
	pcrHash := sha256.Sum256(pcrValue)

	policyPCRCommandValue := make([]byte, 4)
	binary.BigEndian.PutUint32(policyPCRCommandValue, uint32(tpm2.TPMCCPolicyPCR))

	pcrSelectionMarshalled := tpm2.Marshal(pcrSelection)

	commandWithPCRSelectionMarshalled := append(policyPCRCommandValue, pcrSelectionMarshalled...)

	toHash := append(initial[:], append(commandWithPCRSelectionMarshalled, pcrHash[:]...)...)

	hashed := sha256.Sum256(toHash)

	return hashed[:]
}

func parseRSAKey(key string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(key)
	if err != nil {
		return nil, err
	}

	// convert private key to rsa.PrivateKey
	rsaPrivateKeyBlock, _ := pem.Decode(keyData)
	if rsaPrivateKeyBlock == nil {
		return nil, err
	}

	rsaKey, err := x509.ParsePKCS1PrivateKey(rsaPrivateKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key failed: %v", err)
	}

	return rsaKey, nil
}

func calculateSignature(digest []byte, hash crypto.Hash, rsaKey *rsa.PrivateKey) (*signatureData, error) {
	digestToHash := hash.New()
	digestToHash.Write(digest)
	digestHashed := digestToHash.Sum(nil)

	// sign policy digest
	signedData, err := rsaKey.Sign(nil, digestHashed[:], hash)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %v", err)
	}

	return &signatureData{
		Digest:          hex.EncodeToString(digest),
		SignatureBase64: base64.StdEncoding.EncodeToString(signedData),
	}, nil
}

func GenerateSignedPCR(sectionsData SectionsData, rsaKey string) (*PCRData, error) {
	sha1BankData, err := calculatePCRBankData(constants.UKIPCR, tpm2.TPMAlgSHA1, sectionsData, rsaKey)
	if err != nil {
		return nil, err
	}

	sha256BankData, err := calculatePCRBankData(constants.UKIPCR, tpm2.TPMAlgSHA256, sectionsData, rsaKey)
	if err != nil {
		return nil, err
	}

	sha384BankData, err := calculatePCRBankData(constants.UKIPCR, tpm2.TPMAlgSHA384, sectionsData, rsaKey)
	if err != nil {
		return nil, err
	}

	sha512BankData, err := calculatePCRBankData(constants.UKIPCR, tpm2.TPMAlgSHA512, sectionsData, rsaKey)
	if err != nil {
		return nil, err
	}

	return &PCRData{
		SHA1:   sha1BankData,
		SHA256: sha256BankData,
		SHA384: sha384BankData,
		SHA512: sha512BankData,
	}, nil
}

func createPCRSelection(s []int) ([]byte, error) {

	const sizeOfPCRSelect = 3

	PCRs := make(tpmutil.RawBytes, sizeOfPCRSelect)

	for _, n := range s {
		if n >= 8*sizeOfPCRSelect {
			return nil, fmt.Errorf("PCR index %d is out of range (exceeds maximum value %d)", n, 8*sizeOfPCRSelect-1)
		}
		byteNum := n / 8
		bytePos := byte(1 << (n % 8))
		PCRs[byteNum] |= bytePos
	}

	return PCRs, nil
}
