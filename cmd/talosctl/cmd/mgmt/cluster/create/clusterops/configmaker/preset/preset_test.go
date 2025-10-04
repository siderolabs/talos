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
			name:       "secureboot without iso",
			presets:    []string{preset.SecureBoot{}.Name()},
			shouldFail: true,
		},
		{
			name:       "secureboot with iso",
			presets:    []string{preset.ISO{}.Name(), preset.SecureBoot{}.Name()},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := preset.ValidatePresets(tt.presets, preset.Options{
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

func TestApplyPXE(t *testing.T) {
	imageFactoryURL, err := url.Parse(constants.ImageFactoryURL)
	require.NoError(t, err)

	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.TargetArch = "arm64"

	err = preset.Apply(preset.Options{
		SchematicID:     constants.ImageFactoryEmptySchematicID,
		ImageFactoryURL: imageFactoryURL,
	}, &cOps, &qOps, []string{preset.PXE{}.Name()})
	require.NoError(t, err)

	expectedPXEBootScript := constants.ImageFactoryURL + "pxe/" + constants.ImageFactoryEmptySchematicID + "/" + cOps.TalosVersion + "/metal-arm64"
	require.Equal(t, expectedPXEBootScript, qOps.NodeIPXEBootScript)
	require.False(t, qOps.Tpm2Enabled)
	require.Empty(t, qOps.NodeISOPath)
}

func TestApplySecureboot(t *testing.T) {
	imageFactoryURL, err := url.Parse(constants.ImageFactoryURL)
	require.NoError(t, err)

	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()
	qOps.TargetArch = "arm64"

	err = preset.Apply(preset.Options{
		SchematicID:     constants.ImageFactoryEmptySchematicID,
		ImageFactoryURL: imageFactoryURL,
	}, &cOps, &qOps, []string{preset.ISO{}.Name(), preset.SecureBoot{}.Name()})
	require.NoError(t, err)

	expectedISOPath := constants.ImageFactoryURL + "image/" + constants.ImageFactoryEmptySchematicID + "/" + cOps.TalosVersion + "/metal-" + qOps.TargetArch + "-secureboot.iso"
	require.Equal(t, expectedISOPath, qOps.NodeISOPath)
	require.True(t, qOps.Tpm2Enabled)
	require.Contains(t, qOps.DiskEncryptionKeyTypes, "tpm")
	require.True(t, qOps.EncryptEphemeralPartition)
	require.True(t, qOps.EncryptStatePartition)

	expectedInstallerPath := constants.ImageFactoryURL + "metal-installer-secureboot/" + constants.ImageFactoryEmptySchematicID + ":" + cOps.TalosVersion
	require.Equal(t, expectedInstallerPath, qOps.NodeInstallImage)
}
