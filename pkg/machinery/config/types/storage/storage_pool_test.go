// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
)

func TestStoragePoolRoundTrip(t *testing.T) {
	t.Parallel()

	const document = "apiVersion: v1alpha1\nkind: StoragePool\nname: vm-images\nvolume:\n    name: u-vm-data\n"

	provider, err := configloader.NewFromBytes([]byte(document))
	require.NoError(t, err)
	require.Len(t, provider.StoragePoolConfigs(), 1)
	pool := provider.StoragePoolConfigs()[0]
	assert.Equal(t, "vm-images", pool.Name())
	assert.Equal(t, "u-vm-data", pool.VolumeName())

	encoded, err := encoder.NewEncoder(pool, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)
	assert.Equal(t, document, string(encoded))

	clone := provider.Documents()[0].Clone().(*storagecfg.StoragePoolV1Alpha1)
	clone.VolumeConfig.VolumeName = "other"
	assert.Equal(t, "u-vm-data", pool.VolumeName())
}

func TestStoragePoolVolumeIDValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		err  string
	}{
		{name: "u-vm-data"},
		{name: "e-vm-data"},
		{name: "x-vm-data"},
		{name: "u-u-vm-data"},
		{name: "", err: "volume.name is required"},
		{name: "vm-data", err: "volume.name must be a runtime volume ID"},
		{name: "r-vm-data", err: "volume.name must be a runtime volume ID"},
		{name: "s-vm-data", err: "volume.name must be a runtime volume ID"},
		{name: "U-vm-data", err: "volume.name must be a runtime volume ID"},
		{name: "u", err: "volume.name must be a runtime volume ID"},
		{name: "u-", err: "volume.name must be a runtime volume ID"},
		{name: "e-", err: "volume.name must be a runtime volume ID"},
		{name: "x-", err: "volume.name must be a runtime volume ID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pool := storagecfg.NewStoragePoolV1Alpha1()
			pool.MetaName = "vm-images"
			pool.VolumeConfig.VolumeName = test.name

			_, err := pool.Validate(validationMode{})
			if test.err != "" {
				require.ErrorContains(t, err, test.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStoragePoolValidate(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"vm-images", "VM_images", "a", strings.Repeat("a", 63)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool := storagecfg.NewStoragePoolV1Alpha1()
			pool.MetaName = name
			pool.VolumeConfig.VolumeName = "u-vm-data"
			warnings, err := pool.Validate(validationMode{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
		})
	}

	for _, name := range []string{"", ".", "..", "../escape", "/absolute", "a/b", "a\\b", "with space", "ümlaut", "a\x00b", "-option", strings.Repeat("a", 64)} {
		t.Run("invalid-"+name, func(t *testing.T) {
			t.Parallel()

			pool := storagecfg.NewStoragePoolV1Alpha1()
			pool.MetaName = name
			pool.VolumeConfig.VolumeName = "u-vm-data"
			_, err := pool.Validate(validationMode{})
			require.ErrorContains(t, err, "name must be")
		})
	}

	pool := storagecfg.NewStoragePoolV1Alpha1()
	pool.MetaName = "vm-images"
	_, err := pool.Validate(validationMode{})
	require.ErrorContains(t, err, "volume.name is required")
}
