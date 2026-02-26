// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/blackholerouteconfig.yaml
var expectedBlackholeRouteConfigDocument []byte

func TestBlackholeRouteConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewBlackholeRouteConfigV1Alpha1("169.254.1.1/32")
	cfg.RouteMetric = 2000

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedBlackholeRouteConfigDocument, marshaled)
}

func TestBlackholeRouteConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedBlackholeRouteConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.BlackholeRouteConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.BlackholeRouteKind,
		},
		MetaName:    "169.254.1.1/32",
		RouteMetric: 2000,
	}, docs[0])
}

func TestBlackholeRouteConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.BlackholeRouteConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.BlackholeRouteConfigV1Alpha1 {
				return network.NewBlackholeRouteConfigV1Alpha1("")
			},

			expectedError: "name must be specified\nname must be a valid address prefix: netip.ParsePrefix(\"\"): no '/'",
		},
		{
			name: "invalid prefix",

			cfg: func() *network.BlackholeRouteConfigV1Alpha1 {
				cfg := network.NewBlackholeRouteConfigV1Alpha1("no-prefix")

				return cfg
			},

			expectedError: "name must be a valid address prefix: netip.ParsePrefix(\"no-prefix\"): no '/'",
		},
		{
			name: "valid",

			cfg: func() *network.BlackholeRouteConfigV1Alpha1 {
				return network.NewBlackholeRouteConfigV1Alpha1("10.0.1.2/24")
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
