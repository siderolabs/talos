// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"os"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpmutil"

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

// SealedResponse is the response from the TPM2.0 Seal operation.
type SealedResponse struct {
	SealedBlobPrivate []byte
	SealedBlobPublic  []byte
	KeyName           []byte
	PolicyDigest      []byte
}

// Seal seals the key using TPM2.0.
func Seal(key []byte) (*SealedResponse, error) {
	t, err := transport.OpenTPM()
	if err != nil {
		return nil, err
	}
	defer t.Close() // nolint: errcheck

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

// Unseal unseals a sealed blob using the TPM
// nolint:gocyclo,cyclop
func Unseal(sealed SealedResponse) ([]byte, error) {
	t, err := transport.OpenTPM()
	if err != nil {
		return nil, err
	}
	defer t.Close() // nolint: errcheck

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
		return nil, fmt.Errorf("srk name does not match")
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
		return nil, fmt.Errorf("failed to create policy session: %v", err)
	}

	defer policyCloseFunc() // nolint: errcheck

	pubKey, err := parsePCRSigningPubKey()
	if err != nil {
		return nil, err
	}

	loadExternal := tpm2.LoadExternal{
		Hierarchy: tpm2.TPMRHOwner,
		InPublic:  tpm2.New2B(pubKeyTemplate(pubKey.N.BitLen(), pubKey.E, pubKey.N.Bytes())),
	}

	loadExternalResponse, err := loadExternal.Execute(t)
	if err != nil {
		return nil, fmt.Errorf("failed to load external key: %v", err)
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

	pcrSelector, err := createPCRSelection([]int{constants.UKIMeasuredPCR})
	if err != nil {
		return nil, err
	}

	policyPCR := tpm2.PolicyPCR{
		PolicySession: policySess.Handle(),
		Pcrs: tpm2.TPMLPCRSelection{
			PCRSelections: []tpm2.TPMSPCRSelection{
				{
					Hash:      tpm2.TPMAlgSHA256,
					PCRSelect: pcrSelector,
				},
			},
		},
	}

	_, err = policyPCR.Execute(t)
	if err != nil {
		return nil, fmt.Errorf("failed to execute policy pcr: %v", err)
	}

	policyGetDigest := tpm2.PolicyGetDigest{
		PolicySession: policySess.Handle(),
	}

	policyGetDigestResp, err := policyGetDigest.Execute(t)
	if err != nil {
		return nil, fmt.Errorf("failed to execute policy get digest: %v", err)
	}

	sigJSON, err := parsePCRSignature()
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

		if bytes.Equal(digest, policyGetDigestResp.PolicyDigest.Buffer) {
			signature = bank.Sig

			if hex.EncodeToString(pubKeyFingerprint[:]) != bank.PKFP {
				return nil, fmt.Errorf("certificate fingerprint does not match")
			}

			break
		}
	}

	if signature == "" {
		return nil, fmt.Errorf("signature not found")
	}

	signatureDecoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, err
	}

	// Verify will only verify the RSA part of the RSA+SHA256 signature,
	// hence we need to do the SHA256 part ourselves
	policyDigestHash := sha256.Sum256(policyGetDigestResp.PolicyDigest.Buffer)

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
		return nil, fmt.Errorf("failed to verify signature: %v", err)
	}

	policyAuthorize := tpm2.PolicyAuthorize{
		PolicySession:  policySess.Handle(),
		ApprovedPolicy: policyGetDigestResp.PolicyDigest,
		KeySign:        loadExternalResponse.Name,
		CheckTicket:    verifySignatureResponse.Validation,
	}

	if _, err = policyAuthorize.Execute(t); err != nil {
		return nil, fmt.Errorf("failed to execute policy authorize: %v", err)
	}

	// we need to call policyPCR again to update the policy digest
	if _, err = policyPCR.Execute(t); err != nil {
		return nil, fmt.Errorf("failed to execute policy pcr: %v", err)
	}

	policyGetDigestResp, err = policyGetDigest.Execute(t)
	if err != nil {
		return nil, fmt.Errorf("failed to execute policy get digest: %v", err)
	}

	if !bytes.Equal(policyGetDigestResp.PolicyDigest.Buffer, sealed.PolicyDigest) {
		return nil, fmt.Errorf("sealing policy digest does not match")
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
		return nil, fmt.Errorf("failed to unseal op: %v", err)
	}

	return unsealResponse.OutData.Buffer, nil
}

// PCRExtent hashes the input and extends the PCR with the hash.
func PCRExtent(pcr int, data []byte) error {
	t, err := transport.OpenTPM()
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("TPM device is not available, skipping PCR extension")

			return nil
		}

		return err
	}

	defer t.Close() // nolint: errcheck

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

func pubKeyTemplate(bitlen, exponent int, modulus []byte) tpm2.TPMTPublic {
	return tpm2.TPMTPublic{
		Type:    tpm2.TPMAlgRSA,
		NameAlg: tpm2.TPMAlgSHA256,
		ObjectAttributes: tpm2.TPMAObject{
			Decrypt:      true,
			SignEncrypt:  true,
			UserWithAuth: true,
		},
		Parameters: tpm2.NewTPMUPublicParms(tpm2.TPMAlgRSA, &tpm2.TPMSRSAParms{
			Symmetric: tpm2.TPMTSymDefObject{
				Algorithm: tpm2.TPMAlgNull,
				Mode:      tpm2.NewTPMUSymMode(tpm2.TPMAlgRSA, tpm2.TPMAlgNull),
			},
			Scheme: tpm2.TPMTRSAScheme{
				Scheme: tpm2.TPMAlgNull,
				Details: tpm2.NewTPMUAsymScheme(tpm2.TPMAlgRSA, &tpm2.TPMSSigSchemeRSASSA{
					HashAlg: tpm2.TPMAlgNull,
				}),
			},
			KeyBits:  tpm2.TPMKeyBits(bitlen),
			Exponent: uint32(exponent),
		}),
		Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{
			Buffer: modulus,
		}),
	}
}

func calculateSealingPolicyDigest(t transport.TPM) ([]byte, error) {
	policyAuthorizationDigest, err := calculatePolicyAuthorizationDigest()
	if err != nil {
		return nil, err
	}

	pcrSelector, err := createPCRSelection([]int{constants.UKIMeasuredPCR})
	if err != nil {
		return nil, fmt.Errorf("failed to create PCR selection: %v", err)
	}

	pcrValue, err := readPCR(t, constants.UKIMeasuredPCR)
	if err != nil {
		return nil, err
	}

	sealingDigest := calculatePolicyPCR(policyAuthorizationDigest, pcrValue, tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      tpm2.TPMAlgSHA256,
				PCRSelect: pcrSelector,
			},
		},
	})

	return sealingDigest, nil
}

func calculatePolicyAuthorizationDigest() ([]byte, error) {
	tpm2PubKey, err := parsePCRSigningPubKey()
	if err != nil {
		return nil, err
	}

	publicKeyTemplate := pubKeyTemplate(tpm2PubKey.N.BitLen(), tpm2PubKey.E, tpm2PubKey.N.Bytes())

	name, err := tpm2.ObjectName(&publicKeyTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate name: %v", err)
	}

	policyAuthorizeCommand := make([]byte, 4)
	binary.BigEndian.PutUint32(policyAuthorizeCommand, uint32(tpm2.TPMCCPolicyAuthorize))

	// PolicyAuthorize does not use the previous hash value
	// start with all zeros
	initial := bytes.Repeat([]byte{0x00}, sha256.Size)

	initial = append(initial, policyAuthorizeCommand...)
	initial = append(initial, name.Buffer...)

	policyAuthorizeInitialDigest := sha256.Sum256(initial)

	// PolicyAuthorize requires hashing twice
	policyAuthorizeDigest := sha256.Sum256(policyAuthorizeInitialDigest[:])

	return policyAuthorizeDigest[:], nil
}

func parsePCRSigningPubKey() (*rsa.PublicKey, error) {
	pcrSigningPubKey, err := os.ReadFile(constants.PCRPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read pcr signing public key: %v", err)
	}

	block, _ := pem.Decode(pcrSigningPubKey)
	if block == nil {
		return nil, fmt.Errorf("failed to decode pcr signing public key")
	}

	// parse rsa public key
	tpm2PubKeyAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	tpm2PubKey, ok := tpm2PubKeyAny.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to cast pcr signing public key to rsa")
	}

	return tpm2PubKey, nil
}

func parsePCRSignature() (*PCRData, error) {
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

func calculatePolicyPCR(policyAuthorizeDigest, pcrValue []byte, pcrSelection tpm2.TPMLPCRSelection) []byte {
	pcrHash := sha256.Sum256(pcrValue)

	policyPCRCommandValue := make([]byte, 4)
	binary.BigEndian.PutUint32(policyPCRCommandValue, uint32(tpm2.TPMCCPolicyPCR))

	pcrSelectionMarshalled := tpm2.Marshal(pcrSelection)

	pcrToHash := make([]byte, 0)

	pcrToHash = append(pcrToHash, policyAuthorizeDigest...)
	pcrToHash = append(pcrToHash, policyPCRCommandValue...)
	pcrToHash = append(pcrToHash, pcrSelectionMarshalled...)
	pcrToHash = append(pcrToHash, pcrHash[:]...)

	pcrDigest := sha256.Sum256(pcrToHash)

	return pcrDigest[:]
}

// nolint:gocyclo
func validatePCRBanks(t transport.TPM) error {
	pcrValue, err := readPCR(t, constants.UKIMeasuredPCR)
	if err != nil {
		return fmt.Errorf("failed to read PCR: %v", err)
	}

	if bytes.Equal(pcrValue, bytes.Repeat([]byte{0x00}, sha256.Size)) {
		return fmt.Errorf("PCR bank %d is populated with zeroes", constants.UKIMeasuredPCR)
	}

	if bytes.Equal(pcrValue, bytes.Repeat([]byte{0xFF}, sha256.Size)) {
		return fmt.Errorf("PCR bank %d is populated with 0xFF", constants.UKIMeasuredPCR)
	}

	caps := tpm2.GetCapability{
		Capability:    tpm2.TPMCapPCRs,
		Property:      0,
		PropertyCount: 1,
	}

	capsResp, err := caps.Execute(t)
	if err != nil {
		return fmt.Errorf("failed to get PCR capabilities: %v", err)
	}

	assignedPCRs, err := capsResp.CapabilityData.Data.AssignedPCR()
	if err != nil {
		return fmt.Errorf("failed to parse assigned PCRs: %v", err)
	}

	for _, s := range assignedPCRs.PCRSelections {
		h, err := s.Hash.Hash()
		if err != nil {
			return fmt.Errorf("failed to parse hash algorithm: %v", err)
		}

		switch h { //nolint:exhaustive
		case crypto.SHA1:
			continue
		case crypto.SHA256:
			// check if 24 banks are available
			if len(s.PCRSelect) != 24/8 {
				return fmt.Errorf("unexpected number of PCR banks: %d", len(s.PCRSelect))
			}

			// check if all banks are available
			if s.PCRSelect[0] != 0xff || s.PCRSelect[1] != 0xff || s.PCRSelect[2] != 0xff {
				return fmt.Errorf("unexpected PCR banks: %v", s.PCRSelect)
			}
		case crypto.SHA384:
			continue
		case crypto.SHA512:
			continue
		default:
			return fmt.Errorf("unsupported hash algorithm: %s", h.String())
		}
	}

	return nil
}

func readPCR(t transport.TPM, pcr int) ([]byte, error) {
	pcrSelector, err := createPCRSelection([]int{pcr})
	if err != nil {
		return nil, fmt.Errorf("failed to create PCR selection: %v", err)
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
		return nil, fmt.Errorf("failed to read PCR: %v", err)
	}

	return pcrValue.PCRValues.Digests[0].Buffer, nil
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
