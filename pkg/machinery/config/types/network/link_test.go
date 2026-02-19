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
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/linkconfig.yaml
var expectedLinkConfigDocument []byte

func TestLinkConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewLinkConfigV1Alpha1("enp0s1")
	cfg.LinkMTU = 9000
	cfg.LinkUp = new(true)
	cfg.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("192.168.1.100/24"),
		},
		{
			AddressAddress:  netip.MustParsePrefix("2001:db8::1/64"),
			AddressPriority: new(uint32(100)),
		},
	}
	cfg.LinkRoutes = []network.RouteConfig{
		{
			RouteDestination: network.Prefix{netip.MustParsePrefix("10.3.5.0/24")},
			RouteGateway:     network.Addr{netip.MustParseAddr("10.3.5.1")},
		},
		{
			RouteGateway: network.Addr{netip.MustParseAddr("fe80::1")},
		},
	}
	cfg.LinkMulticast = new(true)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedLinkConfigDocument, marshaled)
}

func TestLinkConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedLinkConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.LinkConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.LinkKind,
		},
		MetaName: "enp0s1",
		CommonLinkConfig: network.CommonLinkConfig{
			LinkMTU: 9000,
			LinkUp:  new(true),
			LinkAddresses: []network.AddressConfig{
				{
					AddressAddress: netip.MustParsePrefix("192.168.1.100/24"),
				},
				{
					AddressAddress:  netip.MustParsePrefix("2001:db8::1/64"),
					AddressPriority: new(uint32(100)),
				},
			},
			LinkRoutes: []network.RouteConfig{
				{
					RouteDestination: network.Prefix{netip.MustParsePrefix("10.3.5.0/24")},
					RouteGateway:     network.Addr{netip.MustParseAddr("10.3.5.1")},
				},
				{
					RouteGateway: network.Addr{netip.MustParseAddr("fe80::1")},
				},
			},
			LinkMulticast: new(true),
		},
	}, docs[0])
}

func TestLinkValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.LinkConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.LinkConfigV1Alpha1 {
				return network.NewLinkConfigV1Alpha1("")
			},

			expectedError: "name must be specified",
		},
		{
			name: "invalid addresses",

			cfg: func() *network.LinkConfigV1Alpha1 {
				cfg := network.NewLinkConfigV1Alpha1("enp0s2")
				cfg.LinkAddresses = []network.AddressConfig{
					{
						AddressAddress: netip.Prefix{},
					},
					{
						AddressAddress: netip.MustParsePrefix("0.0.0.0/0"),
					},
				}

				return cfg
			},

			expectedError: "address 0 must be specified\naddress 1 must be a valid IP address",
		},
		{
			name: "valid",
			cfg: func() *network.LinkConfigV1Alpha1 {
				cfg := network.NewLinkConfigV1Alpha1("enp0s2")
				cfg.LinkMTU = 9000
				cfg.LinkUp = new(true)
				cfg.LinkAddresses = []network.AddressConfig{
					{
						AddressAddress: netip.MustParsePrefix("192.168.1.100/24"),
					},
					{
						AddressAddress: netip.MustParsePrefix("fd00::1/64"),
					},
				}
				cfg.LinkRoutes = []network.RouteConfig{
					{
						RouteDestination: network.Prefix{netip.MustParsePrefix("10.3.5.0/24")},
						RouteGateway:     network.Addr{netip.MustParseAddr("10.3.5.1")},
					},
					{
						RouteGateway: network.Addr{netip.MustParseAddr("fe80::1")},
					},
				}

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
