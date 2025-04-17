// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

//docgen:jsonschema

// EncryptionSpec represents volume encryption settings.
//
//	examples:
//	  - value: exampleEncryptionSpec()
type EncryptionSpec struct {
	//   description: >
	//     Encryption provider to use for the encryption.
	//   values:
	//     - luks2
	EncryptionProvider block.EncryptionProviderType `yaml:"provider"`
	//   description: >
	//     Defines the encryption keys generation and storage method.
	EncryptionKeys []EncryptionKey `yaml:"keys"`
	//   description: >
	//     Cipher to use for the encryption.
	//     Depends on the encryption provider.
	//   values:
	//     - aes-xts-plain64
	//     - xchacha12,aes-adiantum-plain64
	//     - xchacha20,aes-adiantum-plain64
	//   examples:
	//     - value: '"aes-xts-plain64"'
	EncryptionCipher string `yaml:"cipher,omitempty"`
	//   description: >
	//     Defines the encryption key length.
	EncryptionKeySize uint `yaml:"keySize,omitempty"`
	//   description: >
	//     Defines the encryption sector size.
	//   examples:
	//     - value: '4096'
	EncryptionBlockSize uint64 `yaml:"blockSize,omitempty"`
	//   description: >
	//     Additional --perf parameters for the LUKS2 encryption.
	//   values:
	//     - no_read_workqueue
	//     - no_write_workqueue
	//     - same_cpu_crypt
	//   examples:
	//     -  value: >
	//          []string{"no_read_workqueue","no_write_workqueue"}
	EncryptionPerfOptions []string `yaml:"options,omitempty"`
}

func exampleEncryptionSpec() *EncryptionSpec {
	return &EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []EncryptionKey{
			{
				KeySlot: 0,
				KeyStatic: &EncryptionKeyStatic{
					KeyData: "exampleKey",
				},
			},
			{
				KeySlot: 1,
				KeyKMS: &EncryptionKeyKMS{
					KMSEndpoint: "https://example-kms-endpoint.com",
				},
			},
		},
		EncryptionCipher:    "aes-xts-plain64",
		EncryptionBlockSize: 4096,
	}
}

// EncryptionKey represents configuration for disk encryption key.
type EncryptionKey struct {
	//   description: >
	//     Key slot number for LUKS2 encryption.
	KeySlot int `yaml:"slot"`
	//   description: >
	//     Key which value is stored in the configuration file.
	KeyStatic *EncryptionKeyStatic `yaml:"static,omitempty"`
	//   description: >
	//     Deterministically generated key from the node UUID and PartitionLabel.
	KeyNodeID *EncryptionKeyNodeID `yaml:"nodeID,omitempty"`
	//   description: >
	//     KMS managed encryption key.
	KeyKMS *EncryptionKeyKMS `yaml:"kms,omitempty"`
	//   description: >
	//     Enable TPM based disk encryption.
	KeyTPM *EncryptionKeyTPM `yaml:"tpm,omitempty"`
}

// EncryptionKeyStatic represents throw away key type.
type EncryptionKeyStatic struct {
	//   description: >
	//     Defines the static passphrase value.
	KeyData string `yaml:"passphrase,omitempty"`
}

// EncryptionKeyKMS represents a key that is generated and then sealed/unsealed by the KMS server.
//
//	examples:
//	  - value: exampleKMSKey()
type EncryptionKeyKMS struct {
	//   description: >
	//     KMS endpoint to Seal/Unseal the key.
	KMSEndpoint string `yaml:"endpoint"`
}

// EncryptionKeyTPM represents a key that is generated and then sealed/unsealed by the TPM.
type EncryptionKeyTPM struct {
	//   description: >
	//     Check that Secureboot is enabled in the EFI firmware.
	//
	//     If Secureboot is not enabled, the enrollment of the key will fail.
	//     As the TPM key is anyways bound to the value of PCR 7,
	//     changing Secureboot status or configuration
	//     after the initial enrollment will make the key unusable.
	TPMCheckSecurebootStatusOnEnroll *bool `yaml:"checkSecurebootStatusOnEnroll,omitempty"`
}

// EncryptionKeyNodeID represents deterministically generated key from the node UUID and PartitionLabel.
type EncryptionKeyNodeID struct{}

func exampleKMSKey() *EncryptionKeyKMS {
	return &EncryptionKeyKMS{
		KMSEndpoint: "https://192.168.88.21:4443",
	}
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s EncryptionSpec) Validate() ([]string, error) {
	if s.EncryptionProvider == block.EncryptionProviderNone && len(s.EncryptionKeys) == 0 {
		return nil, nil
	}

	var errs error

	switch s.EncryptionProvider {
	case block.EncryptionProviderLUKS2:
	case block.EncryptionProviderNone:
		fallthrough
	default:
		errs = errors.Join(errs, fmt.Errorf("unsupported encryption provider: %s", s.EncryptionProvider))
	}

	if len(s.EncryptionKeys) == 0 {
		errs = errors.Join(errs, errors.New("encryption keys are required"))
	}

	slotsInUse := make(map[int]struct{}, len(s.EncryptionKeys))

	for _, key := range s.EncryptionKeys {
		if _, ok := slotsInUse[key.KeySlot]; ok {
			errs = errors.Join(errs, fmt.Errorf("duplicate key slot %d", key.KeySlot))
		}

		slotsInUse[key.KeySlot] = struct{}{}

		if key.KeyStatic == nil && key.KeyNodeID == nil && key.KeyKMS == nil && key.KeyTPM == nil {
			errs = errors.Join(errs, fmt.Errorf("at least one encryption key type must be specified for slot %d", key.KeySlot))
		}
	}

	return nil, errs
}

// Provider implements the config.Provider interface.
func (s EncryptionSpec) Provider() block.EncryptionProviderType {
	return s.EncryptionProvider
}

// Cipher implements the config.Provider interface.
func (s EncryptionSpec) Cipher() string {
	return s.EncryptionCipher
}

// KeySize implements the config.Provider interface.
func (s EncryptionSpec) KeySize() uint {
	return s.EncryptionKeySize
}

// BlockSize implements the config.Provider interface.
func (s EncryptionSpec) BlockSize() uint64 {
	return s.EncryptionBlockSize
}

// Options implements the config.Provider interface.
func (s EncryptionSpec) Options() []string {
	return s.EncryptionPerfOptions
}

// Keys implements the config.Provider interface.
func (s EncryptionSpec) Keys() []config.EncryptionKey {
	return xslices.Map(s.EncryptionKeys, func(k EncryptionKey) config.EncryptionKey { return k })
}

// Slot implements the config.Provider interface.
func (k EncryptionKey) Slot() int {
	return k.KeySlot
}

// Static implements the config.Provider interface.
func (k EncryptionKey) Static() config.EncryptionKeyStatic {
	if k.KeyStatic == nil {
		return nil
	}

	return k.KeyStatic
}

// NodeID implements the config.Provider interface.
func (k EncryptionKey) NodeID() config.EncryptionKeyNodeID {
	if k.KeyNodeID == nil {
		return nil
	}

	return k.KeyNodeID
}

// KMS implements the config.Provider interface.
func (k EncryptionKey) KMS() config.EncryptionKeyKMS {
	if k.KeyKMS == nil {
		return nil
	}

	return k.KeyKMS
}

// TPM implements the config.Provider interface.
func (k EncryptionKey) TPM() config.EncryptionKeyTPM {
	if k.KeyTPM == nil {
		return nil
	}

	return k.KeyTPM
}

// String implements the config.Provider interface.
func (e *EncryptionKeyNodeID) String() string {
	return "nodeid"
}

// String implements the config.Provider interface.
func (e *EncryptionKeyTPM) String() string {
	return "tpm"
}

// CheckSecurebootOnEnroll implements the config.Provider interface.
func (e *EncryptionKeyTPM) CheckSecurebootOnEnroll() bool {
	if e == nil {
		return false
	}

	return pointer.SafeDeref(e.TPMCheckSecurebootStatusOnEnroll)
}

// Key implements the config.Provider interface.
func (e *EncryptionKeyStatic) Key() []byte {
	return []byte(e.KeyData)
}

// String implements the config.Provider interface.
func (e *EncryptionKeyStatic) String() string {
	return "static"
}

// Endpoint implements the config.Provider interface.
func (e *EncryptionKeyKMS) Endpoint() string {
	return e.KMSEndpoint
}

// String implements the config.Provider interface.
func (e *EncryptionKeyKMS) String() string {
	return "kms"
}
