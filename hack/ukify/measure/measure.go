// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package measure

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/google/go-tpm-tools/simulator"
	// TODO: frezbo: switch to new tpm2 package.
	// Ref: https://github.com/google/go-tpm/releases/tag/v0.9.0
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"

	"github.com/siderolabs/ukify/constants"
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

func calculatePCRBankData(pcr int, alg tpm2.Algorithm, sectionData SectionsData, privateKeyFile string) ([]bankData, error) {
	rsaKey, err := parseRSAKey(privateKeyFile)
	if err != nil {
		return nil, err
	}

	// get fingerprint of public key
	pubKeyFingerprint := sha256.Sum256(x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey))

	sim, err := simulator.Get()
	if err != nil {
		return nil, fmt.Errorf("creating tpm2 simulator failed: %v", err)
	}

	defer sim.Close()

	for _, section := range constants.OrderedSections() {
		if file, ok := sectionData[section]; ok && file != "" {
			if err := pcrExtent(sim, pcr, alg, append([]byte(section), 0)); err != nil {
				return nil, err
			}

			sectionData, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}

			if err := pcrExtent(sim, pcr, alg, sectionData); err != nil {
				return nil, err
			}
		}
	}

	banks := make([]bankData, len(constants.OrderedPhases()))

	for i, phase := range constants.OrderedPhases() {
		if err := pcrExtent(sim, pcr, alg, []byte(phase)); err != nil {
			return nil, err
		}

		sigData, err := calculateSignature(sim, rsaKey, pcr, alg)
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

func calculateSignature(rw io.ReadWriter, rsaKey *rsa.PrivateKey, pcr int, alg tpm2.Algorithm) (*signatureData, error) {
	pcrData, err := tpm2.ReadPCR(rw, pcr, alg)
	if err != nil {
		return nil, fmt.Errorf("reading pcr failed: %v", err)
	}

	pcrHash := sha256.Sum256(pcrData)

	tpm2Session, _, err := tpm2.StartAuthSession(
		rw,
		tpm2.HandleNull,
		tpm2.HandleNull,
		make([]byte, 16),
		nil,
		tpm2.SessionTrial,
		tpm2.AlgNull,
		// session hash alorithm is always SHA256
		tpm2.AlgSHA256,
	)
	if err != nil {
		return nil, err
	}

	defer tpm2.FlushContext(rw, tpm2Session)

	sel := tpm2.PCRSelection{
		Hash: alg,
		PCRs: []int{pcr},
	}

	if err := tpm2.PolicyPCR(rw, tpm2Session, pcrHash[:], sel); err != nil {
		return nil, err
	}

	policyDigest, err := tpm2.PolicyGetDigest(rw, tpm2Session)
	if err != nil {
		return nil, err
	}

	policyDigestHashed, err := hashFromAlg(alg, policyDigest)
	if err != nil {
		return nil, err
	}

	sigHash, err := alg.Hash()
	if err != nil {
		return nil, err
	}

	// sign policy digest
	signedData, err := rsaKey.Sign(nil, policyDigestHashed, sigHash)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %v", err)
	}

	return &signatureData{
		Digest:          hex.EncodeToString(policyDigest[:]),
		SignatureBase64: base64.StdEncoding.EncodeToString(signedData),
	}, nil
}

func hashFromAlg(alg tpm2.Algorithm, data []byte) ([]byte, error) {
	signHash, err := alg.Hash()
	if err != nil {
		return nil, err
	}

	switch signHash.String() {
	case crypto.SHA1.String():
		digest := sha1.Sum(data)

		return digest[:], nil
	case crypto.SHA256.String():
		digest := sha256.Sum256(data)

		return digest[:], nil
	case crypto.SHA384.String():
		digest := sha512.Sum384(data)

		return digest[:], nil
	case crypto.SHA512.String():
		digest := sha512.Sum512(data)

		return digest[:], nil
	}

	return nil, fmt.Errorf("unsupported hash algorithm: %v", signHash)
}

// pcrExtent hashes the input and extends the PCR with the hash
func pcrExtent(rw io.ReadWriter, pcr int, alg tpm2.Algorithm, data []byte) error {
	// we can't use tpm2.Hash here since it's buffer size is too limited
	// ref: https://github.com/google/go-tpm/blob/3270509f088425fc9499bc9b7b8ff0811119bedb/tpm2/constants.go#L47
	digest, err := hashFromAlg(alg, data)
	if err != nil {
		return err
	}

	return tpm2.PCRExtend(rw, tpmutil.Handle(pcr), alg, digest, "")
}

func GenerateSignedPCR(sectionsData SectionsData, rsaKey string) (*PCRData, error) {
	sha1BankData, err := calculatePCRBankData(constants.UKIPCR, tpm2.AlgSHA1, sectionsData, rsaKey)
	if err != nil {
		return nil, err
	}

	sha256BankData, err := calculatePCRBankData(constants.UKIPCR, tpm2.AlgSHA256, sectionsData, rsaKey)
	if err != nil {
		return nil, err
	}

	sha384BankData, err := calculatePCRBankData(constants.UKIPCR, tpm2.AlgSHA384, sectionsData, rsaKey)
	if err != nil {
		return nil, err
	}

	sha512BankData, err := calculatePCRBankData(constants.UKIPCR, tpm2.AlgSHA512, sectionsData, rsaKey)
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
