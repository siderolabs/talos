// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl,goconst
package block_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestSwapVolumeConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *block.SwapVolumeConfigV1Alpha1
	}{
		{
			name:     "disk selector",
			filename: "swapvolumeconfig_diskselector.yaml",
			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = "big"

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.transport == "nvme" && !system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustByteSize("100GiB")

				return c
			},
		},
		{
			name:     "encrypted",
			filename: "swapvolumeconfig_encrypted.yaml",
			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = "secret-swap"

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
				c.EncryptionSpec.EncryptionCipher = "aes-xts-plain64"
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
					{
						KeySlot: 1,
						KeyStatic: &block.EncryptionKeyStatic{
							KeyData: "topsecret",
						},
					},
				}

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			warnings, err := cfg.Validate(validationMode{})
			require.NoError(t, err)
			require.Empty(t, warnings)

			marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
			require.NoError(t, err)

			t.Log(string(marshaled))

			expectedMarshaled, err := os.ReadFile(filepath.Join("testdata", test.filename))
			require.NoError(t, err)

			assert.Equal(t, string(expectedMarshaled), string(marshaled))

			provider, err := configloader.NewFromBytes(expectedMarshaled)
			require.NoError(t, err)

			docs := provider.Documents()
			require.Len(t, docs, 1)

			assert.Equal(t, cfg, docs[0])
		})
	}
}

func TestSwapVolumeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *block.SwapVolumeConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "no name",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 1u`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")

				return c
			},

			expectedErrors: "name is required\nname must be between 1 and 34 characters long",
		},
		{
			name: "too long name",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = strings.Repeat("X", 35)

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 1u`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")

				return c
			},

			expectedErrors: "name must be between 1 and 34 characters long",
		},
		{
			name: "invalid characters in name",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = "some/name"

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 1u`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")

				return c
			},

			expectedErrors: "name can only contain lowercase and uppercase ASCII letters, digits, and hyphens",
		},
		{
			name: "invalid disk selector",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120`)))

				return c
			},

			expectedErrors: "disk selector is invalid: ERROR: <input>:1:11: found no matching overload for '_>_' applied to '(uint, int)'\n | disk.size > 120\n | ..........^\nmin size or max size is required",
		},
		{
			name: "min size greater than max size",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustByteSize("10GiB")

				return c
			},

			expectedErrors: "disk selector is required\nmin size is greater than max size",
		},
		{
			name: "no encryption provider",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
				}

				return c
			},

			expectedErrors: "unsupported encryption provider: none",
		},
		{
			name: "no encryption keys",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2

				return c
			},

			expectedErrors: "encryption keys are required",
		},
		{
			name: "invalid encryption key slots",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 1,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
					{
						KeySlot: 0,
					},
					{
						KeySlot: 1,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
				}

				return c
			},

			expectedErrors: "at least one encryption key type must be specified for slot 0\nduplicate key slot 1",
		},
		{
			name: "valid",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120u * GiB`)))
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustByteSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")

				return c
			},
		},
		{
			name: "valid encrypted",

			cfg: func(t *testing.T) *block.SwapVolumeConfigV1Alpha1 {
				c := block.NewSwapVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
				c.EncryptionSpec.EncryptionCipher = "aes-xts-plain64"
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
				}

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			_, err := cfg.Validate(validationMode{})

			if test.expectedErrors == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				assert.EqualError(t, err, test.expectedErrors)
			}
		})
	}
}
