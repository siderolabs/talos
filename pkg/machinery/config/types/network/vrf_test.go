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
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed testdata/vrfconfig.yaml
var expectedVRFConfigDocument []byte

func TestVRFConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewVRFConfigV1Alpha1("vrf-blue")
	cfg.VRFLinks = []string{"eno1", "eno5"}
	cfg.VRFTable = 123
	cfg.LinkUp = new(true)
	cfg.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("1.2.3.5/32"),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedVRFConfigDocument, marshaled)
}

func TestVRFConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedVRFConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.VRFConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.VRFKind,
		},
		MetaName: "vrf-blue",
		VRFLinks: []string{"eno1", "eno5"},
		VRFTable: 123,
		CommonLinkConfig: network.CommonLinkConfig{
			LinkUp: new(true),
			LinkAddresses: []network.AddressConfig{
				{
					AddressAddress: netip.MustParsePrefix("1.2.3.5/32"),
				},
			},
		},
	}, docs[0])
}

func TestVRFValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.VRFConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.VRFConfigV1Alpha1 {
				return network.NewVRFConfigV1Alpha1("")
			},
			expectedError: "name must be specified\ncannot create vrf with reserved table unspec",
		},
		{
			name: "reject reserved table id 0",
			cfg: func() *network.VRFConfigV1Alpha1 {
				cfg := network.NewVRFConfigV1Alpha1("vrf-blue")

				return cfg
			},
			expectedError: "cannot create vrf with reserved table unspec",
		},
		{
			name: "reject reserved table id 253",
			cfg: func() *network.VRFConfigV1Alpha1 {
				cfg := network.NewVRFConfigV1Alpha1("vrf-blue")
				cfg.VRFTable = nethelpers.TableDefault

				return cfg
			},
			expectedError: "cannot create vrf with reserved table default",
		},
		{
			name: "reject reserved table id 254",
			cfg: func() *network.VRFConfigV1Alpha1 {
				cfg := network.NewVRFConfigV1Alpha1("vrf-blue")
				cfg.VRFTable = nethelpers.TableMain

				return cfg
			},
			expectedError: "cannot create vrf with reserved table main",
		},
		{
			name: "reject reserved table id 255",
			cfg: func() *network.VRFConfigV1Alpha1 {
				cfg := network.NewVRFConfigV1Alpha1("vrf-blue")
				cfg.VRFTable = nethelpers.TableLocal

				return cfg
			},
			expectedError: "cannot create vrf with reserved table local",
		},
		{
			name: "valid",
			cfg: func() *network.VRFConfigV1Alpha1 {
				cfg := network.NewVRFConfigV1Alpha1("vrf-red")
				cfg.VRFLinks = []string{"eth0", "eth1"}
				cfg.VRFTable = nethelpers.Table123
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
