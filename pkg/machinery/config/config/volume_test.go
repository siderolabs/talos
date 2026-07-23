// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

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
