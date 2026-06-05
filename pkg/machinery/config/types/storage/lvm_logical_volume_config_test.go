// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst
package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

//nolint:dupl
func TestLVMLogicalVolumeConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1
	}{
		{
			name:     "basic",
			filename: "lvmlogicalvolumeconfig_basic.yaml",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.LVType = storageres.LVMLogicalVolumeTypeLinear
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

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

func TestLVMLogicalVolumeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "no name",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.LVType = storageres.LVMLogicalVolumeTypeLinear
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

				return c
			},
			expectedErrors: "name is required\nname must be between 1 and 63 characters long",
		},
		{
			name: "missing volume group",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

				return c
			},
			expectedErrors: "provisioning.volumeGroup is required",
		},
		{
			name: "missing max size",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.Provisioning.VolumeGroup = "vg-pool"

				return c
			},
			expectedErrors: "provisioning.maxSize is required",
		},
		{
			name: "min greater than max",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMinSize = block.MustByteSize("100GiB")
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

				return c
			},
			expectedErrors: "provisioning.minSize must not exceed provisioning.maxSize",
		},
		{
			name: "valid percent",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.LVType = storageres.LVMLogicalVolumeTypeRAID1
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("80%")

				return c
			},
		},
		{
			name: "mirrors on linear",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.LVMirrors = new(uint32(1))
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

				return c
			},
			expectedErrors: "mirrors is only valid for raid1/raid10, not linear",
		},
		{
			name: "stripes on raid1",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.LVType = storageres.LVMLogicalVolumeTypeRAID1
				c.LVStripes = new(uint32(2))
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

				return c
			},
			expectedErrors: "stripes is only valid for raid0/raid10, not raid1",
		},
		{
			name: "stripes too low on raid0",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.LVType = storageres.LVMLogicalVolumeTypeRAID0
				c.LVStripes = new(uint32(1))
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

				return c
			},
			expectedErrors: "stripes must be at least 2",
		},
		{
			name: "valid raid0 with stripes",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.LVType = storageres.LVMLogicalVolumeTypeRAID0
				c.LVStripes = new(uint32(3))
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

				return c
			},
		},
		{
			name: "valid raid10 with mirrors and stripes",
			cfg: func(t *testing.T) *storagecfg.LVMLogicalVolumeConfigV1Alpha1 {
				c := storagecfg.NewLVMLogicalVolumeConfigV1Alpha1()
				c.MetaName = "lv-data"
				c.LVType = storageres.LVMLogicalVolumeTypeRAID10
				c.LVMirrors = new(uint32(1))
				c.LVStripes = new(uint32(2))
				c.Provisioning.VolumeGroup = "vg-pool"
				c.Provisioning.ProvisioningMaxSize = block.MustSize("50GiB")

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
