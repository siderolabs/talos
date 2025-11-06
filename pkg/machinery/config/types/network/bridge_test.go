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
)

//go:embed testdata/bridgeconfig.yaml
var expectedBridgeConfigDocument []byte

func TestBridgeConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewBridgeConfigV1Alpha1("bridge.1")
	cfg.BridgeLinks = []string{"eno1", "eno5"}
	cfg.BridgeSTP.BridgeSTPEnabled = pointer.To(true)
	cfg.BridgeVLAN.BridgeVLANFiltering = pointer.To(false)
	cfg.LinkUp = pointer.To(true)
	cfg.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("1.2.3.5/32"),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedBridgeConfigDocument, marshaled)
}

func TestBridgeConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedBridgeConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.BridgeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.BridgeKind,
		},
		MetaName:    "bridge.1",
		BridgeLinks: []string{"eno1", "eno5"},
		BridgeSTP:   network.BridgeSTPConfig{BridgeSTPEnabled: pointer.To(true)},
		BridgeVLAN:  network.BridgeVLANConfig{BridgeVLANFiltering: pointer.To(false)},
		CommonLinkConfig: network.CommonLinkConfig{
			LinkUp: pointer.To(true),
			LinkAddresses: []network.AddressConfig{
				{
					AddressAddress: netip.MustParsePrefix("1.2.3.5/32"),
				},
			},
		},
	}, docs[0])
}

func TestBridgeValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.BridgeConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.BridgeConfigV1Alpha1 {
				return network.NewBridgeConfigV1Alpha1("")
			},

			expectedError: "name must be specified",
		},
		{
			name: "no links",

			cfg: func() *network.BridgeConfigV1Alpha1 {
				cfg := network.NewBridgeConfigV1Alpha1("Bridge0")

				return cfg
			},
		},
		{
			name: "valid",
			cfg: func() *network.BridgeConfigV1Alpha1 {
				cfg := network.NewBridgeConfigV1Alpha1("Bridge25")
				cfg.BridgeLinks = []string{"eth0", "eth1"}
				cfg.BridgeSTP.BridgeSTPEnabled = pointer.To(true)
				cfg.BridgeVLAN.BridgeVLANFiltering = pointer.To(true)
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
