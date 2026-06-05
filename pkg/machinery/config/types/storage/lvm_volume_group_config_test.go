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
func TestLVMVolumeGroupConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1
	}{
		{
			name:     "basic",
			filename: "lvmvolumegroupconfig_basic.yaml",
			cfg: func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
				c := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
				c.MetaName = "vg-pool"

				require.NoError(t, c.PhysicalVolumes.VolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "nvme"`)))

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

func TestLVMVolumeGroupConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "no name",

			cfg: func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
				c := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()

				require.NoError(t, c.PhysicalVolumes.VolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "nvme"`)))

				return c
			},

			expectedErrors: "name is required\nname must be between 1 and 63 characters long",
		},
		{
			name: "too long name",

			cfg: func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
				c := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
				c.MetaName = strings.Repeat("X", 64)

				require.NoError(t, c.PhysicalVolumes.VolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "nvme"`)))

				return c
			},

			expectedErrors: "name must be between 1 and 63 characters long",
		},
		{
			name: "invalid characters in name",

			cfg: func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
				c := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
				c.MetaName = "vg pool"

				require.NoError(t, c.PhysicalVolumes.VolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "nvme"`)))

				return c
			},

			expectedErrors: "name can only contain ASCII letters, digits, hyphens and underscores",
		},
		{
			name: "missing selector",

			cfg: func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
				c := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
				c.MetaName = "vg-pool"

				return c
			},

			expectedErrors: "physicalVolumes.volumeSelector.match is required",
		},
		{
			name: "valid",

			cfg: func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
				c := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
				c.MetaName = "vg-pool"

				require.NoError(t, c.PhysicalVolumes.VolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "nvme"`)))

				return c
			},
		},
		{
			name: "valid with underscore in name",

			cfg: func(t *testing.T) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
				c := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
				c.MetaName = "vg_pool_data"

				require.NoError(t, c.PhysicalVolumes.VolumeSelector.Match.UnmarshalText([]byte(`disk.transport == "nvme"`)))

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

func (validationMode) IsAgent() bool {
	return false
}
