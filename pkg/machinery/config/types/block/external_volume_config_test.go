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
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestExternalVolumeConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1
	}{
		{
			name:     "basic virtiofs",
			filename: "externalvolumeconfig_basicvirtiofs.yaml",
			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "my-virtiofs-volume"
				c.FilesystemType = blockres.FilesystemTypeVirtiofs
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

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

func TestExternalVolumeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "no name",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.FilesystemType = blockres.FilesystemTypeVirtiofs
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},

			expectedErrors: "name is required\nname must be between 1 and 34 characters long",
		},
		{
			name: "invalid characters in name",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "some/name"
				c.FilesystemType = blockres.FilesystemTypeVirtiofs
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},

			expectedErrors: "name can only contain lowercase and uppercase ASCII letters, digits, and hyphens",
		},
		{
			name: "no mount spec",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.FilesystemType = blockres.FilesystemTypeVirtiofs

				return c
			},

			expectedErrors: "virtiofs mount spec is required",
		},
		{
			name: "invalid type",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.FilesystemType = blockres.FilesystemTypeEXT4
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},

			expectedErrors: "invalid filesystem type: ext4",
		},
		{
			name: "empty type",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},

			expectedErrors: "invalid filesystem type: none",
		},
		{
			name: "valid virtiofs",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.FilesystemType = blockres.FilesystemTypeVirtiofs
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

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
