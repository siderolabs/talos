package main

// TODO: maybe some of this is useful: https://ericchiang.github.io/post/tpm-keys/

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/simulator"
	"github.com/google/go-tpm/tpmutil"
)

var (
	//go:embed testdata/tpm2-pcr-public.pem
	pcrSigningPubKey []byte
	//go:embed testdata/pcr-data.json
	pcrDataJSON []byte
)

type LuksHeader struct {
	Type            string   `json:"type"`
	Keyslots        []string `json:"keyslots"`
	TPM2PrivateBlob []byte   `json:"tpm2_private_blob"`
	TPM2PublicBlob  []byte   `json:"tpm2_public_blob"`
	TPM2PCRS        []int    `json:"tpm2_pcrs"`
	TPM2Alg         string   `json:"tpm2_alg"`
	TPM2PolicyHash  []byte   `json:"tpm2_policy_hash"`
	TPM2PublicKey   string   `json:"tpm2_public_key"`
	TPM2SRK         []byte   `json:"tpm2_srk"`
}

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

const (
	// The SRK handle is defined in the Provisioning Guidance document (see above) in the table "Reserved Handles
	// for TPM Provisioning Fundamental Elements". The SRK is useful because it is "shared", meaning it has no
	// authValue nor authPolicy set, and thus may be used by anyone on the system to generate derived keys or
	// seal secrets. This is useful if the TPM has an auth (password) set for the 'owner hierarchy', which would
	// prevent users from generating primary transient keys, unless they knew the owner hierarchy auth. See
	// the Provisioning Guidance document for more details.
	// https://trustedcomputinggroup.org/resource/tcg-tpm-v2-0-provisioning-guidance
	// https://trustedcomputinggroup.org/resource/http-trustedcomputinggroup-org-wp-content-uploads-tcg-ek-credential-profile
	TPM2_SRK_HANDLE = tpmutil.Handle(0x81000001)
)

type TPM struct {
	transport io.ReadWriteCloser
}

func (t *TPM) Send(input []byte) ([]byte, error) {
	return tpmutil.RunCommandRaw(t.transport, input)
}

func (t *TPM) Close() error {
	return t.transport.Close()
}

func seal(t transport.TPM, diskEncryptionKey []byte) (*LuksHeader, error) {
	sealingPolicyDigest, err := calculateSealingPolicyDigest(t)
	if err != nil {
		return nil, err
	}

	// TODO: verify the pubkey fingerprint matches the one in the pcr policy json and we have a matching digest for the pcr policy
	// this is strictly not necessary just that unlock will fail

	luks, err := createLuksHeader(t, diskEncryptionKey, sealingPolicyDigest)
	if err != nil {
		return nil, err
	}

	return luks, nil
}

func unseal(t transport.TPM, luks LuksHeader) ([]byte, error) {
	tpmPub, err := tpm2.Unmarshal[tpm2.TPM2BPublic](luks.TPM2PublicBlob)
	if err != nil {
		return nil, err
	}

	tpmPriv, err := tpm2.Unmarshal[tpm2.TPM2BPrivate](luks.TPM2PrivateBlob)
	if err != nil {
		return nil, err
	}

	srk, err := tpm2.Unmarshal[tpm2.TPM2BName](luks.TPM2SRK)
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
		16,
		tpm2.Salted(createPrimaryResponse.ObjectHandle, *outPub),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy session: %v", err)
	}

	defer policyCloseFunc()

	pubKey, err := parsePCRSigningPubKey()
	if err != nil {
		return nil, err
	}

	loadExternal := tpm2.LoadExternal{
		Hierarchy: tpm2.TPMRHOwner,
		InPublic: tpm2.New2B(tpm2.TPMTPublic{
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
				KeyBits:  tpm2.TPMKeyBits(pubKey.N.BitLen()),
				Exponent: uint32(pubKey.E),
			}),
			Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{
				Buffer: pubKey.N.Bytes(),
			}),
		}),
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

	pcrSelector, err := createPCRSelection([]int{11})
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

	var sigJSON PCRData

	if err := json.Unmarshal(pcrDataJSON, &sigJSON); err != nil {
		return nil, err
	}

	// TODO: verify the fingerprint matches the one in the JSON
	// pubKeyFingerprint := sha256.Sum256(x509.MarshalPKCS1PublicKey(pubKey))

	var signature string
	for _, bank := range sigJSON.SHA256 {
		digest, err := hex.DecodeString(bank.POL)
		if err != nil {
			return nil, err
		}

		if bytes.Equal(digest, policyGetDigestResp.PolicyDigest.Buffer) {
			signature = bank.SIG

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

	if !bytes.Equal(policyGetDigestResp.PolicyDigest.Buffer, luks.TPM2PolicyHash) {
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
		16,
		tpm2.Salted(createPrimaryResponse.ObjectHandle, *outPub),
		tpm2.AESEncryption(128, tpm2.EncryptOut),
		tpm2.Bound(loadResponse.ObjectHandle, loadResponse.Name, nil),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to unseal op: %v", err)
	}

	return unsealResponse.OutData.Buffer, nil
}

func main() {
	t, err := simulator.OpenSimulator()
	if err != nil {
		log.Fatal(err)
	}

	defer t.Close()

	/* this can also be used with swtpm:
	rm -rf /tmp/mytpm/ && \
	mkdir -p /tmp/mytpm && \
	swtpm socket \
		--tpmstate dir=/tmp/mytpm \
		--server type=unixio,path=/tmp/mytpm/tpm.sock \
		--tpm2 \
		--flags not-need-init,startup-clear \
		--log level=20
	*/
	// transport, err := tpmutil.OpenTPM("/tmp/mytpm/tpm.sock")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// defer transport.Close()

	// t := &TPM{transport: transport}

	// defer t.Close()

	if err := validatePCRBanks(t); err != nil {
		log.Fatal(err)
	}

	randBytesBase64Encoded := "fov2vtqURStyfHwWMGEbCv0KyOB1mAf/iI4rMdmmK2Q="
	randBytes, err := base64.StdEncoding.DecodeString(randBytesBase64Encoded)
	if err != nil {
		log.Fatalf("failed to decode rand bytes: %v", err)
	}

	luks, err := seal(t, randBytes)
	if err != nil {
		log.Fatalf("failed to seal: %v", err)
	}

	unsealed, err := unseal(t, *luks)
	if err != nil {
		log.Fatalf("failed to unseal: %v", err)
	}

	if !bytes.Equal(randBytes, unsealed) {
		log.Fatalf("unsealed data does not match")
	}
}

func hashFromAlg(alg tpm2.TPMAlgID, data []byte) ([]byte, error) {
	signHash, err := alg.Hash()
	if err != nil {
		return nil, err
	}

	switch signHash {
	case crypto.SHA1:
		digest := sha1.Sum(data)

		return digest[:], nil
	case crypto.SHA256:
		digest := sha256.Sum256(data)

		return digest[:], nil
	case crypto.SHA384:
		digest := sha512.Sum384(data)

		return digest[:], nil
	case crypto.SHA512:
		digest := sha512.Sum512(data)

		return digest[:], nil
	}

	return nil, fmt.Errorf("unsupported hash algorithm: %v", signHash)
}

// pcrExtent hashes the input and extends the PCR with the hash
func pcrExtent(rw transport.TPM, pcr int, alg tpm2.TPMAlgID, data []byte) error {
	// we can't use tpm2.Hash here since it's buffer size is too limited
	// ref: https://github.com/google/go-tpm/blob/3270509f088425fc9499bc9b7b8ff0811119bedb/tpm2/constants.go#L47
	digest, err := hashFromAlg(alg, data)
	if err != nil {
		return err
	}

	pcrHandle := tpm2.PCRExtend{
		PCRHandle: tpm2.AuthHandle{
			Handle: tpm2.TPMHandle(pcr),
			Auth:   tpm2.PasswordAuth(nil),
		},
		Digests: tpm2.TPMLDigestValues{
			Digests: []tpm2.TPMTHA{
				{
					HashAlg: alg,
					Digest:  digest,
				},
			},
		},
	}

	if _, err = pcrHandle.Execute(rw); err != nil {
		return err
	}

	return nil
}

func parsePCRSigningPubKey() (*rsa.PublicKey, error) {
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

func createLuksHeader(t transport.TPM, diskEncryptionKey, sealingPolicyDigest []byte) (*LuksHeader, error) {
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
				16,
				tpm2.Salted(createPrimaryResponse.ObjectHandle, *outPub),
				tpm2.AESEncryption(128, tpm2.EncryptInOut),
			),
		},
		InSensitive: tpm2.TPM2BSensitiveCreate{
			Sensitive: &tpm2.TPMSSensitiveCreate{
				Data: tpm2.NewTPMUSensitiveCreate(&tpm2.TPM2BSensitiveData{
					Buffer: diskEncryptionKey,
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

	luks := &LuksHeader{
		Type:            "talos-tpm2",
		Keyslots:        []string{},
		TPM2PrivateBlob: tpm2.Marshal(createResp.OutPrivate),
		TPM2PublicBlob:  tpm2.Marshal(createResp.OutPublic),
		TPM2PCRS:        []int{11},
		TPM2Alg:         "sha256",
		TPM2PolicyHash:  sealingPolicyDigest,
		TPM2SRK:         tpm2.Marshal(createPrimaryResponse.Name),
	}

	return luks, nil
}

func calculateSealingPolicyDigest(t transport.TPM) ([]byte, error) {
	policyAuthorizationDigest, err := calculatePolicyAuthorizationDigest(t)
	if err != nil {
		return nil, err
	}

	pcrSelector, err := createPCRSelection([]int{11})
	if err != nil {
		return nil, fmt.Errorf("failed to create PCR selection: %v", err)
	}

	pcrValue, err := readPCR(t, 11)
	if err != nil {
		return nil, err
	}

	sealingDigest, err := calculatePolicyPCR(t, policyAuthorizationDigest, pcrValue, tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      tpm2.TPMAlgSHA256,
				PCRSelect: pcrSelector,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return sealingDigest, nil
}

func calculatePolicyAuthorizationDigest(t transport.TPM) ([]byte, error) {
	tpm2PubKey, err := parsePCRSigningPubKey()
	if err != nil {
		return nil, err
	}

	publicKeyTemplate := tpm2.TPMTPublic{
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
			KeyBits:  tpm2.TPMKeyBits(tpm2PubKey.N.BitLen()),
			Exponent: uint32(tpm2PubKey.E),
		}),
		Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{
			Buffer: tpm2PubKey.N.Bytes(),
		}),
	}

	name, err := tpm2.ObjectName(&publicKeyTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate name: %v", err)
	}

	bs := make([]byte, 4) // TODO: should I use the length of command here?
	binary.BigEndian.PutUint32(bs, uint32(tpm2.TPMCCPolicyAuthorize))

	// PolicyAuthorize does not use the previous hash value
	// start with all zeros
	initial := bytes.Repeat([]byte{0x00}, 32)

	e := append(initial, append(bs, name.Buffer...)...)

	s := sha256.Sum256(e)

	// PolicyAuthorize requires hashing twice
	rehash := sha256.Sum256(s[:])

	return rehash[:], nil
}

func calculatePolicyPCR(t transport.TPM, policyAuthorizeDigest, pcrValue []byte, pcrSelection tpm2.TPMLPCRSelection) ([]byte, error) {
	pcrHash := sha256.Sum256(pcrValue)

	policyPCRCommandValue := make([]byte, 4)
	binary.BigEndian.PutUint32(policyPCRCommandValue, uint32(tpm2.TPMCCPolicyPCR))

	pcrSelectionMarshalled := tpm2.Marshal(pcrSelection)

	commandWithPCRSelectionMarshalled := append(policyPCRCommandValue, pcrSelectionMarshalled...)

	toHash := append(policyAuthorizeDigest, append(commandWithPCRSelectionMarshalled, pcrHash[:]...)...)

	pcrDigest := sha256.Sum256(toHash)

	return pcrDigest[:], nil
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

func validatePCRBanks(t transport.TPM) error {
	pcrValue, err := readPCR(t, 11)
	if err != nil {
		return fmt.Errorf("failed to read PCR: %v", err)
	}

	if bytes.Equal(pcrValue, bytes.Repeat([]byte{0x00}, 32)) {
		// TODO this is for testing only, remove and fail if PCR is zero
		if err := extendPCRWithKnownValues(t); err != nil {
			return fmt.Errorf("failed to extend PCR with known values: %v", err)
		}

		// log.Fatalf("PCR bank %d is populated with zeroes", 11)
	}

	if bytes.Equal(pcrValue, bytes.Repeat([]byte{0xFF}, 32)) {
		return fmt.Errorf("PCR bank %d is populated with 0xFF", 11)
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

		switch h {
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

func extendPCRWithKnownValues(t transport.TPM) error {
	if err := pcrExtent(t, 11, tpm2.TPMAlgSHA256, append([]byte(".linux"), []byte{0x00}...)); err != nil {
		return err
	}

	if err := pcrExtent(t, 11, tpm2.TPMAlgSHA256, []byte("hello\n")); err != nil {
		return err
	}

	if err := pcrExtent(t, 11, tpm2.TPMAlgSHA256, []byte("enter-initrd")); err != nil {
		return err
	}

	return nil
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
