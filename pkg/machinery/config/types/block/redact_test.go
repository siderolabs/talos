// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func encryptedSpec() block.EncryptionSpec {
	return block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot:   0,
				KeyStatic: &block.EncryptionKeyStatic{KeyData: "static-passphrase-0"},
			},
			{
				KeySlot:   1,
				KeyNodeID: &block.EncryptionKeyNodeID{},
			},
			{
				KeySlot:   2,
				KeyStatic: &block.EncryptionKeyStatic{KeyData: "static-passphrase-2"},
			},
		},
	}
}

func TestEncryptionSpecRedact(t *testing.T) {
	t.Parallel()

	spec := encryptedSpec()
	spec.Redact("REDACTED")

	assert.Equal(t, "REDACTED", spec.EncryptionKeys[0].KeyStatic.KeyData)
	assert.Nil(t, spec.EncryptionKeys[1].KeyStatic)
	assert.Equal(t, "REDACTED", spec.EncryptionKeys[2].KeyStatic.KeyData)
}

func TestVolumeConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := block.NewVolumeConfigV1Alpha1()
	cfg.MetaName = "EPHEMERAL"
	cfg.EncryptionSpec = encryptedSpec()

	cfg.Redact("REDACTED")

	assert.Equal(t, "REDACTED", cfg.EncryptionSpec.EncryptionKeys[0].KeyStatic.KeyData)
	assert.Equal(t, "REDACTED", cfg.EncryptionSpec.EncryptionKeys[2].KeyStatic.KeyData)
}

func TestRawVolumeConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := block.NewRawVolumeConfigV1Alpha1()
	cfg.MetaName = "raw1"
	cfg.EncryptionSpec = encryptedSpec()

	cfg.Redact("REDACTED")

	assert.Equal(t, "REDACTED", cfg.EncryptionSpec.EncryptionKeys[0].KeyStatic.KeyData)
	assert.Equal(t, "REDACTED", cfg.EncryptionSpec.EncryptionKeys[2].KeyStatic.KeyData)
}

func TestUserVolumeConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := block.NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = "user1"
	cfg.EncryptionSpec = encryptedSpec()

	cfg.Redact("REDACTED")

	assert.Equal(t, "REDACTED", cfg.EncryptionSpec.EncryptionKeys[0].KeyStatic.KeyData)
	assert.Equal(t, "REDACTED", cfg.EncryptionSpec.EncryptionKeys[2].KeyStatic.KeyData)
}

func TestSwapVolumeConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := block.NewSwapVolumeConfigV1Alpha1()
	cfg.MetaName = "swap1"
	cfg.EncryptionSpec = encryptedSpec()

	cfg.Redact("REDACTED")

	assert.Equal(t, "REDACTED", cfg.EncryptionSpec.EncryptionKeys[0].KeyStatic.KeyData)
	assert.Equal(t, "REDACTED", cfg.EncryptionSpec.EncryptionKeys[2].KeyStatic.KeyData)
}

func TestInlineDataVolumeConfigRedact(t *testing.T) {
	t.Parallel()

	newConfig := func(secret bool) *block.InlineDataVolumeConfigV1Alpha1 {
		cfg := block.NewInlineDataVolumeConfigV1Alpha1()
		cfg.MetaName = "app"
		cfg.SecretConfig = secret
		cfg.FilesConfig = map[string]block.InlineDataVolumeFileV1Alpha1{
			"password":   {FileMode: new(block.InlineDataVolumeFileMode(0o600)), FileContents: "hunter2"},
			"sub/token":  {FileContents: "abc"},
			"empty.conf": {},
		}

		return cfg
	}

	t.Run("secret", func(t *testing.T) {
		t.Parallel()

		cfg := newConfig(true)
		cfg.Redact("REDACTED")

		assert.Equal(t, map[string]block.InlineDataVolumeFileV1Alpha1{
			"REDACTED-0": {},
			"REDACTED-1": {FileMode: new(block.InlineDataVolumeFileMode(0o600)), FileContents: "REDACTED"},
			"REDACTED-2": {FileContents: "REDACTED"},
		}, cfg.FilesConfig)

		_, err := cfg.Validate(validationMode{})
		assert.NoError(t, err)
	})

	t.Run("not secret", func(t *testing.T) {
		t.Parallel()

		cfg := newConfig(false)
		cfg.Redact("REDACTED")

		assert.Equal(t, newConfig(false).FilesConfig, cfg.FilesConfig)
	})
}
