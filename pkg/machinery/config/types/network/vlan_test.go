// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"net/netip"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed testdata/vlanconfig.yaml
var expectedVLANConfigDocument []byte

func TestVLANConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewVLANConfigV1Alpha1("enp0s3.2")
	cfg.VLANIDConfig = 2
	cfg.ParentLinkConfig = "enp0s3"
	cfg.VLANModeConfig = pointer.To(nethelpers.VLANProtocol8021Q)
	cfg.LinkUp = pointer.To(true)
	cfg.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("192.168.1.100/32"),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedVLANConfigDocument, marshaled)
}

func TestVLANConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedVLANConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.VLANConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.VLANKind,
		},
		MetaName:         "enp0s3.2",
		VLANIDConfig:     2,
		ParentLinkConfig: "enp0s3",
		VLANModeConfig:   pointer.To(nethelpers.VLANProtocol8021Q),
		CommonLinkConfig: network.CommonLinkConfig{
			LinkUp: pointer.To(true),
			LinkAddresses: []network.AddressConfig{
				{
					AddressAddress: netip.MustParsePrefix("192.168.1.100/32"),
				},
			},
		},
	}, docs[0])
}

func TestVLANValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.VLANConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.VLANConfigV1Alpha1 {
				return network.NewVLANConfigV1Alpha1("")
			},

			expectedError: "name must be specified\nvlanID must be specified and between 1 and 4094\nparent must be specified",
		},
		{
			name: "invalid addresses",

			cfg: func() *network.VLANConfigV1Alpha1 {
				cfg := network.NewVLANConfigV1Alpha1("enx8.2")
				cfg.VLANIDConfig = 2
				cfg.ParentLinkConfig = "enx8"
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
			name: "no VLAN ID",

			cfg: func() *network.VLANConfigV1Alpha1 {
				cfg := network.NewVLANConfigV1Alpha1("enx8.2")
				cfg.ParentLinkConfig = "enx9"

				return cfg
			},

			expectedError: "vlanID must be specified and between 1 and 4094",
		},
		{
			name: "high VLAN ID",

			cfg: func() *network.VLANConfigV1Alpha1 {
				cfg := network.NewVLANConfigV1Alpha1("enx8.2")
				cfg.ParentLinkConfig = "enx9"
				cfg.VLANIDConfig = 5000

				return cfg
			},

			expectedError: "vlanID must be specified and between 1 and 4094",
		},
		{
			name: "valid",
			cfg: func() *network.VLANConfigV1Alpha1 {
				cfg := network.NewVLANConfigV1Alpha1("enx8.2")
				cfg.VLANIDConfig = 2
				cfg.ParentLinkConfig = "enx8"
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
