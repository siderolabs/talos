// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/constants"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/preset"
)

func TestValidatePresets(t *testing.T) {
	imageFactoryURL, err := url.Parse(constants.ImageFactoryURL)
	require.NoError(t, err)

	tests := []struct {
		name       string
		presets    []string
		shouldFail bool
	}{
		{
			name:       "no presets",
			presets:    []string{},
			shouldFail: true,
		},
		{
			name:       "multiple boot method presets",
			presets:    []string{preset.ISO{}.Name(), preset.PXE{}.Name()},
			shouldFail: true,
		},
		{
			name:       "valid single boot method preset - iso",
			presets:    []string{preset.ISO{}.Name()},
			shouldFail: false,
		},
		{
			name:       "valid single boot method preset - pxe",
			presets:    []string{preset.PXE{}.Name()},
			shouldFail: false,
		},
		{
			name:       "valid single boot method preset - disk-image",
			presets:    []string{preset.DiskImage{}.Name()},
			shouldFail: false,
		},
		{
			name:       "valid boot method preset with maintenance",
			presets:    []string{preset.ISO{}.Name(), preset.Maintenance{}.Name()},
			shouldFail: false,
		},
		{
			name:       "iso-secureboot",
			presets:    []string{preset.ISOSecureBoot{}.Name()},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := preset.Validate(tt.presets, preset.Options{
				SchematicID:     constants.ImageFactoryEmptySchematicID,
				ImageFactoryURL: imageFactoryURL,
			})

			if tt.shouldFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func applyPreset(t *testing.T, presets ...string) (clusterops.Common, clusterops.Qemu) {
	imageFactoryURL, err := url.Parse(constants.ImageFactoryURL)
	require.NoError(t, err)

	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.TargetArch = "arm64"
	cOps.TalosVersion = "v9.9.9"

	err = preset.Apply(preset.Options{
		SchematicID:     "123schematic123",
		ImageFactoryURL: imageFactoryURL,
	}, &cOps, &qOps, presets)
	require.NoError(t, err)

	return cOps, qOps
}

func TestPXE(t *testing.T) {
	_, qOps := applyPreset(t, preset.PXE{}.Name())

	require.Equal(t, "factory.talos.dev/metal-installer/123schematic123:v9.9.9", qOps.NodeInstallImage)
	require.Equal(t, "https://factory.talos.dev/pxe/123schematic123/v9.9.9/metal-arm64", qOps.NodeIPXEBootScript)
	require.False(t, qOps.Tpm2Enabled)
	require.Empty(t, qOps.NodeISOPath)
}

func TestSecureboot(t *testing.T) {
	_, qOps := applyPreset(t, preset.ISOSecureBoot{}.Name())

	require.Equal(t, "https://factory.talos.dev/image/123schematic123/v9.9.9/metal-arm64-secureboot.iso", qOps.NodeISOPath)
	require.True(t, qOps.Tpm2Enabled)
	require.Contains(t, qOps.DiskEncryptionKeyTypes, "tpm")
	require.True(t, qOps.EncryptEphemeralPartition)
	require.True(t, qOps.EncryptStatePartition)

	require.Equal(t, "factory.talos.dev/metal-installer-secureboot/123schematic123:v9.9.9", qOps.NodeInstallImage)
}

func TestDiskImage(t *testing.T) {
	_, qOps := applyPreset(t, preset.DiskImage{}.Name())

	require.Equal(t, "https://factory.talos.dev/image/123schematic123/v9.9.9/metal-arm64.raw.zst", qOps.NodeDiskImagePath)
}

func TestMaintenance(t *testing.T) {
	cOps, _ := applyPreset(t, preset.Maintenance{}.Name(), preset.ISO{}.Name())

	require.True(t, cOps.SkipInjectingConfig)
	require.False(t, cOps.ApplyConfigEnabled)
}
