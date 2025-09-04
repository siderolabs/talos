// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/statichostconfig.yaml
var expectedStaticHostConfigDocument []byte

func TestStaticHostMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewStaticHostConfigV1Alpha1("10.5.0.2")
	cfg.Hostnames = []string{"example.org", "example.com"}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedStaticHostConfigDocument, marshaled)
}

func TestHostConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.StaticHostConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.StaticHostConfigV1Alpha1 {
				return network.NewStaticHostConfigV1Alpha1("")
			},

			expectedError: "name is required\nname must be a valid IP address\nat least one hostname is required",
		},
		{
			name: "invalid ip",
			cfg: func() *network.StaticHostConfigV1Alpha1 {
				cfg := network.NewStaticHostConfigV1Alpha1("ab")
				cfg.Hostnames = []string{"example.org", "example.com"}

				return cfg
			},

			expectedError: "name must be a valid IP address",
		},
		{
			name: "no hostnames",
			cfg: func() *network.StaticHostConfigV1Alpha1 {
				return network.NewStaticHostConfigV1Alpha1("10.5.0.2")
			},

			expectedError: "at least one hostname is required",
		},
		{
			name: "valid",
			cfg: func() *network.StaticHostConfigV1Alpha1 {
				cfg := network.NewStaticHostConfigV1Alpha1("10.5.0.2")
				cfg.Hostnames = []string{"example.org", "example.com"}

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
