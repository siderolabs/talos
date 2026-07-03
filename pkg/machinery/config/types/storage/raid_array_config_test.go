// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst
package storage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
)

//nolint:dupl
func TestRAIDArrayConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1
	}{
		{
			name:     "basic",
			filename: "raidarrayconfig_basic.yaml",
			cfg: func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1 {
				c := storagecfg.NewRAIDArrayConfigV1Alpha1()
				c.MetaName = "data"

				require.NoError(t, c.ProvisioningSpec.RAIDVolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "virtio"`)))

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

func TestRAIDArrayConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "no name",

			cfg: func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1 {
				c := storagecfg.NewRAIDArrayConfigV1Alpha1()

				require.NoError(t, c.ProvisioningSpec.RAIDVolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "virtio"`)))

				return c
			},

			expectedErrors: "name is required\nname must be between 1 and 32 characters long",
		},
		{
			name: "too long name",

			cfg: func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1 {
				c := storagecfg.NewRAIDArrayConfigV1Alpha1()
				c.MetaName = strings.Repeat("X", 33)

				require.NoError(t, c.ProvisioningSpec.RAIDVolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "virtio"`)))

				return c
			},

			expectedErrors: "name must be between 1 and 32 characters long",
		},
		{
			name: "invalid characters in name",

			cfg: func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1 {
				c := storagecfg.NewRAIDArrayConfigV1Alpha1()
				c.MetaName = "invalid name"

				require.NoError(t, c.ProvisioningSpec.RAIDVolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "virtio"`)))

				return c
			},

			expectedErrors: "name can only contain ASCII letters, digits, hyphens and underscores",
		},
		{
			name: "missing selector",

			cfg: func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1 {
				c := storagecfg.NewRAIDArrayConfigV1Alpha1()
				c.MetaName = "data"

				return c
			},

			expectedErrors: "provisioning.volumeSelector.match is required",
		},
		{
			name: "valid",

			cfg: func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1 {
				c := storagecfg.NewRAIDArrayConfigV1Alpha1()
				c.MetaName = "data"

				require.NoError(t, c.ProvisioningSpec.RAIDVolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "virtio"`)))

				return c
			},
		},
		{
			name: "valid with underscore in name",

			cfg: func(t *testing.T) *storagecfg.RAIDArrayConfigV1Alpha1 {
				c := storagecfg.NewRAIDArrayConfigV1Alpha1()
				c.MetaName = "test_data"

				require.NoError(t, c.ProvisioningSpec.RAIDVolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "virtio"`)))

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
