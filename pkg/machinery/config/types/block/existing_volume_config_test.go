// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl,goconst
package block_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestExistingVolumeConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *block.ExistingVolumeConfigV1Alpha1
	}{
		{
			name:     "with selector",
			filename: "existingvolumeconfig_selector.yaml",
			cfg: func(t *testing.T) *block.ExistingVolumeConfigV1Alpha1 {
				c := block.NewExistingVolumeConfigV1Alpha1()
				c.MetaName = "my-lovely-volume"

				require.NoError(t, c.VolumeDiscoverySpec.VolumeSelectorConfig.Match.UnmarshalText([]byte(`volume.partition_label == "MY-DATA"`)))

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

func TestExistingVolumeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *block.ExistingVolumeConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "no name",

			cfg: func(t *testing.T) *block.ExistingVolumeConfigV1Alpha1 {
				c := block.NewExistingVolumeConfigV1Alpha1()

				require.NoError(t, c.VolumeDiscoverySpec.VolumeSelectorConfig.Match.UnmarshalText([]byte(`volume.partition_label == "MY-DATA"`)))

				return c
			},

			expectedErrors: "name is required",
		},
		{
			name: "invalid characters in name",

			cfg: func(t *testing.T) *block.ExistingVolumeConfigV1Alpha1 {
				c := block.NewExistingVolumeConfigV1Alpha1()
				c.MetaName = "some/name"

				require.NoError(t, c.VolumeDiscoverySpec.VolumeSelectorConfig.Match.UnmarshalText([]byte(`volume.partition_label == "MY-DATA"`)))

				return c
			},

			expectedErrors: "name can only contain lowercase and uppercase ASCII letters, digits, and hyphens",
		},
		{
			name: "invalid volume selector",

			cfg: func(t *testing.T) *block.ExistingVolumeConfigV1Alpha1 {
				c := block.NewExistingVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.VolumeDiscoverySpec.VolumeSelectorConfig.Match.UnmarshalText([]byte(`volume.partition_label == 3`)))

				return c
			},

			expectedErrors: "volume selector is invalid: ERROR: <input>:1:24: found no matching overload for '_==_' applied to '(string, int)'\n | volume.partition_label == 3\n | .......................^",
		},
		{
			name: "valid",

			cfg: func(t *testing.T) *block.ExistingVolumeConfigV1Alpha1 {
				c := block.NewExistingVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.VolumeDiscoverySpec.VolumeSelectorConfig.Match.UnmarshalText([]byte(`volume.partition_label == "MY-DATA"`)))

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
