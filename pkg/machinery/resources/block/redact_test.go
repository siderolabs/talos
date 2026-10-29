// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// TestVolumeConfigRedaction covers the volume encryption: the static passphrase is the only secret,
// the rest of the encryption configuration, which keys are used and how, stays readable.
func TestVolumeConfigRedaction(t *testing.T) {
	t.Parallel()

	spec := block.VolumeConfigSpec{
		Type: block.VolumeTypePartition,
		Encryption: block.EncryptionSpec{
			Provider: block.EncryptionProviderLUKS2,
			Cipher:   "aes-xts-plain64",
			Keys: []block.EncryptionKey{
				{Slot: 0, Type: block.EncryptionKeyTPM, TPMPCRs: []int{7}},
				{Slot: 1, Type: block.EncryptionKeyStatic, StaticPassphrase: []byte("hunter2")},
				{Slot: 2, Type: block.EncryptionKeyKMS, KMSEndpoint: "https://kms.example.com"},
			},
		},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Equal(t, []byte(constants.Redacted), []byte(redacted.Encryption.Keys[1].StaticPassphrase))
	assert.Equal(t, block.EncryptionKeyStatic, redacted.Encryption.Keys[1].Type)
	assert.Equal(t, spec.Encryption.Keys[0], redacted.Encryption.Keys[0])
	assert.Equal(t, spec.Encryption.Keys[2], redacted.Encryption.Keys[2])
	assert.Equal(t, spec.Encryption.Cipher, redacted.Encryption.Cipher)

	assert.Equal(t, []byte("hunter2"), []byte(spec.Encryption.Keys[1].StaticPassphrase),
		"redacting the copy should not mutate the spec it was copied from")

	assert.False(t, (&block.VolumeConfigSpec{Type: block.VolumeTypePartition}).HasSecrets(resource.Metadata{}))
}
