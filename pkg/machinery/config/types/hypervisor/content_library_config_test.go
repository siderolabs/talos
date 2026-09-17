// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
)

func TestContentLibraryConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func() *hypervisor.ContentLibraryConfigV1Alpha1
	}{
		{
			name:     "user volume",
			filename: "contentlibraryconfig_uservolume.yaml",
			cfg: func() *hypervisor.ContentLibraryConfigV1Alpha1 {
				c := hypervisor.NewContentLibraryConfigV1Alpha1()
				c.MetaName = "my-vm-images-1"
				c.BackingConfig.VolumeName = "vm-images"

				return c
			},
		},
		{
			name:     "external volume",
			filename: "contentlibraryconfig_externalvolume.yaml",
			cfg: func() *hypervisor.ContentLibraryConfigV1Alpha1 {
				c := hypervisor.NewContentLibraryConfigV1Alpha1()
				c.MetaName = "shared"
				c.BackingConfig.VolumeName = "nfs-images"

				return c
			},
		},
		{
			name:     "existing volume",
			filename: "contentlibraryconfig_existingvolume.yaml",
			cfg: func() *hypervisor.ContentLibraryConfigV1Alpha1 {
				c := hypervisor.NewContentLibraryConfigV1Alpha1()
				c.MetaName = "found"
				c.BackingConfig.VolumeName = "found-images"

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

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

func TestContentLibraryConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func() *hypervisor.ContentLibraryConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "empty",
			cfg:  hypervisor.NewContentLibraryConfigV1Alpha1,

			expectedErrors: "name is required\nbacking.volume is required",
		},
		{
			name: "invalid name",
			cfg: func() *hypervisor.ContentLibraryConfigV1Alpha1 {
				c := hypervisor.NewContentLibraryConfigV1Alpha1()
				c.MetaName = "vm images"
				c.BackingConfig.VolumeName = "vm-images"

				return c
			},

			expectedErrors: `name "vm images": name can only contain ASCII letters, digits and hyphens`,
		},
		{
			name: "name too long",
			cfg: func() *hypervisor.ContentLibraryConfigV1Alpha1 {
				c := hypervisor.NewContentLibraryConfigV1Alpha1()
				c.MetaName = strings.Repeat("a", 64)
				c.BackingConfig.VolumeName = "vm-images"

				return c
			},

			expectedErrors: fmt.Sprintf("name %q must be 63 characters or fewer", strings.Repeat("a", 64)),
		},
		{
			// A volume may be declared under a name which looks like an internal ID: the ID is derived
			// from the document kind and the name, so this one is addressed as `u-u-vm-images`.
			name: "volume named like an ID",
			cfg: func() *hypervisor.ContentLibraryConfigV1Alpha1 {
				c := hypervisor.NewContentLibraryConfigV1Alpha1()
				c.MetaName = "images"
				c.BackingConfig.VolumeName = "u-vm-images"

				return c
			},
		},
		{
			name: "invalid volume name",
			cfg: func() *hypervisor.ContentLibraryConfigV1Alpha1 {
				c := hypervisor.NewContentLibraryConfigV1Alpha1()
				c.MetaName = "images"
				c.BackingConfig.VolumeName = "vm images"

				return c
			},

			expectedErrors: `backing.volume "vm images": volume name can only contain ASCII letters, digits and hyphens`,
		},
		{
			name: "valid",
			cfg: func() *hypervisor.ContentLibraryConfigV1Alpha1 {
				c := hypervisor.NewContentLibraryConfigV1Alpha1()
				c.MetaName = "images"
				c.BackingConfig.VolumeName = "vm-images"

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

			_, err := cfg.Validate(validationMode{})

			if test.expectedErrors == "" {
				require.NoError(t, err)
			} else {
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
