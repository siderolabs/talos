// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestVolumeConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *block.VolumeConfigV1Alpha1
	}{
		{
			name:     "empty",
			filename: "volumeconfig_empty.yaml",
			cfg: func(*testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				return c
			},
		},
		{
			name:     "disk selector",
			filename: "volumeconfig_diskselector.yaml",
			cfg: func(t *testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.transport == "nvme" && !system_disk`)))

				return c
			},
		},
		{
			name:     "max size",
			filename: "volumeconfig_maxsize.yaml",
			cfg: func(t *testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				c.ProvisioningSpec.ProvisioningMaxSize = block.MustByteSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")

				return c
			},
		},
		{
			name:     "state",
			filename: "volumeconfig_state.yaml",
			cfg: func(t *testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = constants.StatePartitionLabel

				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
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

			warnings, err := cfg.Validate(validationMode{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
		})
	}
}

func TestVolumeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *block.VolumeConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "wrong name",

			cfg: func(t *testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = "wrong"

				return c
			},

			expectedErrors: "only [\"STATE\" \"EPHEMERAL\" \"IMAGECACHE\"] volumes are supported",
		},
		{
			name: "invalid disk selector",

			cfg: func(t *testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120`)))

				return c
			},

			expectedErrors: "disk selector is invalid: ERROR: <input>:1:11: found no matching overload for '_>_' applied to '(uint, int)'\n | disk.size > 120\n | ..........^",
		},
		{
			name: "min size greater than max size",

			cfg: func(t *testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustByteSize("10GiB")

				return c
			},

			expectedErrors: "min size is greater than max size",
		},
		{
			name: "state provisioning config",

			cfg: func(t *testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = constants.StatePartitionLabel

				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5GiB")
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustByteSize("10GiB")

				return c
			},

			expectedErrors: "provisioning config is not allowed for the \"STATE\" volume",
		},
		{
			name: "valid",

			cfg: func(t *testing.T) *block.VolumeConfigV1Alpha1 {
				c := block.NewVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120u * GiB`)))
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustByteSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")

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

func TestVolumeConfigMerge(t *testing.T) {
	c1 := block.NewVolumeConfigV1Alpha1()
	c1.MetaName = constants.EphemeralPartitionLabel

	require.NoError(t, c1.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120`)))

	c2 := block.NewVolumeConfigV1Alpha1()
	c2.MetaName = constants.EphemeralPartitionLabel

	require.NoError(t, c2.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 150`)))
	require.NoError(t, c2.ProvisioningSpec.ProvisioningMaxSize.UnmarshalText([]byte("2.5TiB")))

	require.NoError(t, merge.Merge(c1, c2))

	assert.Equal(t, c1.ProvisioningSpec.DiskSelectorSpec.Match, c2.ProvisioningSpec.DiskSelectorSpec.Match)
	assert.Equal(t, c1.ProvisioningSpec.ProvisioningMaxSize, c2.ProvisioningSpec.ProvisioningMaxSize)
}

type validationMode struct{}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}

func (validationMode) InContainer() bool {
	return false
}
