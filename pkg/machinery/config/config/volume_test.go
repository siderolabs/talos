// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestResolveBackingVolume(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		kind     string
		name     string
		query    string
		mount    string
		id       string
		readOnly bool
		insecure bool
	}{
		{kind: "UserVolumeConfig", name: "images", query: "images", id: "u-images"},
		{kind: "ExistingVolumeConfig", name: "images", query: "images", id: "e-images"},
		{kind: "ExternalVolumeConfig", name: "images", query: "images", id: "x-images"},
		{kind: "UserVolumeConfig", name: "u-images", query: "u-images", id: "u-u-images"},
		{kind: "ExistingVolumeConfig", name: "e-images", query: "e-images", id: "e-e-images"},
		{kind: "ExternalVolumeConfig", name: "x-images", query: "x-images", id: "x-x-images"},
		{kind: "UserVolumeConfig", name: "images", query: "u-images"},
		{kind: "UserVolumeConfig", name: "images", query: "unknown"},
		{kind: "RawVolumeConfig", name: "images", query: "images"},
		{kind: "SwapVolumeConfig", name: "images", query: "images"},
		{kind: "ExistingVolumeConfig", name: "images", query: "images", mount: "mount:\n  readOnly: true\n", id: "e-images", readOnly: true},
		{kind: "ExternalVolumeConfig", name: "images", query: "images", mount: "mount:\n  readOnly: true\n", id: "x-images", readOnly: true},
		{kind: "UserVolumeConfig", name: "images", query: "images", mount: "mount:\n  secure: false\n", id: "u-images", insecure: true},
		{kind: "ExistingVolumeConfig", name: "images", query: "images", mount: "mount:\n  secure: false\n", id: "e-images", insecure: true},
		{kind: "ExternalVolumeConfig", name: "images", query: "images", mount: "mount:\n  secure: false\n", id: "x-images", insecure: true},
	} {
		t.Run(test.kind+"/"+test.name+"/"+test.query+"/"+test.mount, func(t *testing.T) {
			t.Parallel()

			cfg, err := configloader.NewFromBytes(fmt.Appendf(nil, "apiVersion: v1alpha1\nkind: %s\nname: %s\n%s", test.kind, test.name, test.mount))
			require.NoError(t, err)

			volume, found := config.ResolveBackingVolume(cfg, test.query)
			assert.Equal(t, test.id != "", found)
			// Secure unless the volume opts out, which is the default of every volume kind.
			assert.Equal(t, config.BackingVolume{ID: test.id, ReadOnly: test.readOnly, Secure: test.id != "" && !test.insecure}, volume)
		})
	}
}

func TestEmptyVolumeMountSecureDefault(t *testing.T) {
	t.Parallel()

	volumes := config.WrapVolumesConfigList()

	for _, test := range []struct {
		name   string
		secure bool
	}{
		{
			name: constants.EphemeralPartitionLabel,
		},
		{
			name:   constants.StatePartitionLabel,
			secure: true,
		},
		{
			name:   "FUTURE_VOLUME",
			secure: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			volume, ok := volumes.ByName(test.name)
			require.False(t, ok)
			assert.Equal(t, test.secure, volume.Mount().Secure())
		})
	}
}
