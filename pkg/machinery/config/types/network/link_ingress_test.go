// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/networklinkingressconfig.yaml
var expectedLinkIngressConfigDocument []byte

func TestLinkIngressConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewLinkIngressConfigV1Alpha1("enp0s2.35")
	cfg.DestinationAddressesConfig = []meta.Prefix{{Prefix: netip.MustParsePrefix("1.2.3.4/32")}}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedLinkIngressConfigDocument, marshaled)
}

func TestLinkIngressConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedLinkIngressConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	c := &network.LinkIngressConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.LinkIngressKind,
		},
		MetaName:                   "enp0s2.35",
		DestinationAddressesConfig: []meta.Prefix{{Prefix: netip.MustParsePrefix("1.2.3.4/32")}},
	}

	assert.Equal(t, c, docs[0])
}

func TestLinkIngressValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.LinkIngressConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.LinkIngressConfigV1Alpha1 {
				return network.NewLinkIngressConfigV1Alpha1("")
			},

			expectedError: "link name must be specified",
		},
		{
			name: "no destination addresses",
			cfg: func() *network.LinkIngressConfigV1Alpha1 {
				return network.NewLinkIngressConfigV1Alpha1("enp0s2")
			},
		},
		{
			name: "valid override",
			cfg: func() *network.LinkIngressConfigV1Alpha1 {
				c := network.NewLinkIngressConfigV1Alpha1("enp0s2.37")
				c.DestinationAddressesConfig = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("192.168.10.0/24")},
					{Prefix: netip.MustParsePrefix("2001:db8::/32")},
				}

				return c
			},
		},
		{
			name: "invalid prefix",
			cfg: func() *network.LinkIngressConfigV1Alpha1 {
				c := network.NewLinkIngressConfigV1Alpha1("enp0s2")
				c.DestinationAddressesConfig = []meta.Prefix{{}}

				return c
			},

			expectedError: "destinationAddresses[0]: invalid prefix",
		},
		{
			name: "unmasked prefix",
			cfg: func() *network.LinkIngressConfigV1Alpha1 {
				c := network.NewLinkIngressConfigV1Alpha1("enp0s2")
				c.DestinationAddressesConfig = []meta.Prefix{{Prefix: netip.MustParsePrefix("192.168.10.5/24")}}

				return c
			},

			expectedError: "destinationAddresses[0]: prefix 192.168.10.5/24 must be masked",
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

func TestLinkIngressConfigEmptyDestinationAddressesRoundtrip(t *testing.T) {
	t.Parallel()

	// an explicitly empty list allows no destination at all, which is a different setting from the
	// option being absent, so it has to survive a config round-trip
	cfg := network.NewLinkIngressConfigV1Alpha1("enp0s2.35")
	cfg.DestinationAddressesConfig = []meta.Prefix{}

	ctr, err := container.New(cfg)
	require.NoError(t, err)

	marshaled, err := ctr.Bytes()
	require.NoError(t, err)

	t.Log(string(marshaled))

	provider, err := configloader.NewFromBytes(marshaled)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	linkIngress, ok := docs[0].(*network.LinkIngressConfigV1Alpha1)
	require.True(t, ok)

	assert.NotNil(t, linkIngress.DestinationAddresses(), "an explicitly empty list must not decode as unset")
}
