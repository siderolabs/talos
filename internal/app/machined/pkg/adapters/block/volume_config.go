// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeConfigSpec adapter provides conversion from MountStatus.
//
//nolint:revive,golint
func VolumeConfigSpec(r *block.VolumeConfigSpec) volumeConfigSpec {
	return volumeConfigSpec{
		volumeConfigSpec: r,
	}
}

type volumeConfigSpec struct {
	volumeConfigSpec *block.VolumeConfigSpec
}

// WithRoot adapts VolumeConfigSpec to xfs.Root and calls the provided callback with it.
func (a volumeConfigSpec) ApplyEncryptionConfig(in config.EncryptionConfig) error {
	out := a.volumeConfigSpec

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
