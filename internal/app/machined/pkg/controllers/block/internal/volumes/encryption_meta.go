// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"encoding/json"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	blocktype "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// MarshalEncryptionMeta is a function to persist encryption config to the META value.
func MarshalEncryptionMeta(cfg config.EncryptionConfig) ([]byte, error) {
	return json.Marshal(cfg)
}

// UnmarshalEncryptionMeta is a function to load encryption config from the META value.
func UnmarshalEncryptionMeta(data []byte) (config.EncryptionConfig, error) {
	var encryptionFromMeta blocktype.EncryptionSpec

	if err := json.Unmarshal(data, &encryptionFromMeta); err != nil {
		var legacyEncryption v1alpha1.EncryptionConfig

		if legacyErr := json.Unmarshal(data, &legacyEncryption); legacyErr != nil {
			return nil, fmt.Errorf("error unmarshalling state encryption meta key: %w", err)
		}

		return &legacyEncryption, nil
	}

	return &encryptionFromMeta, nil
}

// ConvertEncryptionConfiguration converts a `config.EncryptionConfig` into a
// `block.EncryptionSpec`, and writes it into `out`.
func ConvertEncryptionConfiguration(in config.EncryptionConfig, out *block.VolumeConfigSpec) error {
	if in == nil {
		out.Encryption = block.EncryptionSpec{}

		return nil
	}

	out.Encryption.Provider = in.Provider()
	out.Encryption.Cipher = in.Cipher()
	out.Encryption.KeySize = in.KeySize()
	out.Encryption.BlockSize = in.BlockSize()
	out.Encryption.PerfOptions = in.Options()

	out.Encryption.Keys = make([]block.EncryptionKey, len(in.Keys()))

	for i, key := range in.Keys() {
		out.Encryption.Keys[i].Slot = key.Slot()
		out.Encryption.Keys[i].LockToSTATE = key.LockToSTATE()

		switch {
		case key.Static() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyStatic
			out.Encryption.Keys[i].StaticPassphrase = key.Static().Key()
		case key.NodeID() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyNodeID
		case key.KMS() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyKMS
			out.Encryption.Keys[i].KMSEndpoint = key.KMS().Endpoint()
		case key.TPM() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyTPM
			out.Encryption.Keys[i].TPMCheckSecurebootStatusOnEnroll = key.TPM().CheckSecurebootOnEnroll()
			out.Encryption.Keys[i].TPMPCRs = key.TPM().PCRs()
			out.Encryption.Keys[i].TPMPubKeyPCRs = key.TPM().PubKeyPCRs()
		default:
			return fmt.Errorf("unsupported encryption key type: slot %d", key.Slot())
		}
	}

	return nil
}
