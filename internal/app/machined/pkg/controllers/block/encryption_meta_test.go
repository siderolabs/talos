// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func legacyEncryptionConfig() config.EncryptionConfig {
	return &v1alpha1.EncryptionConfig{
		EncryptionCipher: "aes-xts-plain64",
		EncryptionKeys: []*v1alpha1.EncryptionKey{
			{
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "secret",
				},
			},
			{
				KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
			},
		},
	}
}

func modernEncryptionConfig() config.EncryptionConfig {
	return blockcfg.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionCipher:   "aes-xts-plain64",
		EncryptionKeys: []blockcfg.EncryptionKey{
			{
				KeyStatic: &blockcfg.EncryptionKeyStatic{
					KeyData: "secret",
				},
			},
			{
				KeyNodeID: &blockcfg.EncryptionKeyNodeID{},
			},
		},
	}
}

func assertEqualEncryptionConfigs(t *testing.T, a, b config.EncryptionConfig) {
	t.Helper()

	require.NotNil(t, a)
	require.NotNil(t, b)

	assert.Equal(t, a.Provider(), b.Provider())
	assert.Equal(t, a.Cipher(), b.Cipher())
	assert.Equal(t, a.KeySize(), b.KeySize())
	assert.Equal(t, a.BlockSize(), b.BlockSize())
	assert.ElementsMatch(t, a.Options(), b.Options())

	require.Equal(t, len(a.Keys()), len(b.Keys()))

	for i := range a.Keys() {
		assert.Equal(t, a.Keys()[i].Slot(), b.Keys()[i].Slot())
		assert.Equal(t, a.Keys()[i].Static(), b.Keys()[i].Static())
		assert.Equal(t, a.Keys()[i].NodeID(), b.Keys()[i].NodeID())
		assert.Equal(t, a.Keys()[i].KMS(), b.Keys()[i].KMS())
		assert.Equal(t, a.Keys()[i].TPM(), b.Keys()[i].TPM())
		assert.Equal(t, a.Keys()[i].LockToSTATE(), b.Keys()[i].LockToSTATE())
	}
}

//nolint:lll
const (
	legacyMarshalled = `{"EncryptionProvider":"","EncryptionKeys":[{"KeyStatic":{"KeyData":"secret"},"KeyNodeID":null,"KeyKMS":null,"KeySlot":0,"KeyTPM":null},{"KeyStatic":null,"KeyNodeID":{},"KeyKMS":null,"KeySlot":0,"KeyTPM":null}],"EncryptionCipher":"aes-xts-plain64","EncryptionKeySize":0,"EncryptionBlockSize":0,"EncryptionPerfOptions":null}`
	modernMarshalled = `{"EncryptionProvider":"luks2","EncryptionKeys":[{"KeySlot":0,"KeyStatic":{"KeyData":"secret"},"KeyNodeID":null,"KeyKMS":null,"KeyTPM":null,"KeyLockToSTATE":null},{"KeySlot":0,"KeyStatic":null,"KeyNodeID":{},"KeyKMS":null,"KeyTPM":null,"KeyLockToSTATE":null}],"EncryptionCipher":"aes-xts-plain64","EncryptionKeySize":0,"EncryptionBlockSize":0,"EncryptionPerfOptions":null}`
)

func TestMarshalEncryptionMeta(t *testing.T) {
	t.Parallel()

	data, err := block.MarshalEncryptionMeta(legacyEncryptionConfig())
	require.NoError(t, err)

	assert.Equal(t, legacyMarshalled, string(data))

	data, err = block.MarshalEncryptionMeta(modernEncryptionConfig())
	require.NoError(t, err)

	assert.Equal(t, modernMarshalled, string(data))
}

func TestUnmarshalEncryptionMeta(t *testing.T) {
	t.Parallel()

	cfg, err := block.UnmarshalEncryptionMeta([]byte(legacyMarshalled))
	require.NoError(t, err)

	assertEqualEncryptionConfigs(t, cfg, legacyEncryptionConfig())

	cfg, err = block.UnmarshalEncryptionMeta([]byte(modernMarshalled))
	require.NoError(t, err)

	assertEqualEncryptionConfigs(t, cfg, modernEncryptionConfig())
}
