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

//go:embed testdata/bondconfig.yaml
var expectedBondConfigDocument []byte

func TestBondConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewBondConfigV1Alpha1("agg.0")
	cfg.BondLinks = []string{"eth0", "eth1"}
	cfg.BondMode = new(nethelpers.BondMode8023AD)
	cfg.BondXmitHashPolicy = new(nethelpers.BondXmitPolicyLayer34)
	cfg.BondFailOverMAC = new(nethelpers.FailOverMACFollow)
	cfg.BondLACPRate = new(nethelpers.LACPRateSlow)
	cfg.BondMIIMon = new(uint32(100))
	cfg.BondUpDelay = new(uint32(200))
	cfg.BondDownDelay = new(uint32(200))
	cfg.BondResendIGMP = new(uint32(1))
	cfg.BondPacketsPerSlave = new(uint32(1))
	cfg.BondADActorSysPrio = new(uint16(65535))
	cfg.LinkUp = new(true)
	cfg.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("1.2.3.4/24"),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedBondConfigDocument, marshaled)
}

func TestBondConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedBondConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.BondConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.BondKind,
		},
		MetaName:            "agg.0",
		BondLinks:           []string{"eth0", "eth1"},
		BondMode:            new(nethelpers.BondMode8023AD),
		BondXmitHashPolicy:  new(nethelpers.BondXmitPolicyLayer34),
		BondFailOverMAC:     new(nethelpers.FailOverMACFollow),
		BondLACPRate:        new(nethelpers.LACPRateSlow),
		BondMIIMon:          new(uint32(100)),
		BondUpDelay:         new(uint32(200)),
		BondDownDelay:       new(uint32(200)),
		BondResendIGMP:      new(uint32(1)),
		BondPacketsPerSlave: new(uint32(1)),
		BondADActorSysPrio:  new(uint16(65535)),
		CommonLinkConfig: network.CommonLinkConfig{
			LinkUp: new(true),
			LinkAddresses: []network.AddressConfig{
				{
					AddressAddress: netip.MustParsePrefix("1.2.3.4/24"),
				},
			},
		},
	}, docs[0])
}

func TestBondValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.BondConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.BondConfigV1Alpha1 {
				return network.NewBondConfigV1Alpha1("")
			},

			expectedError: "name must be specified\nat least one link must be specified\nbond mode must be specified",
		},
		{
			name: "no links",

			cfg: func() *network.BondConfigV1Alpha1 {
				cfg := network.NewBondConfigV1Alpha1("bond0")
				cfg.BondMode = new(nethelpers.BondModeActiveBackup)

				return cfg
			},

			expectedError: "at least one link must be specified",
		},
		{
			name: "no mode",

			cfg: func() *network.BondConfigV1Alpha1 {
				cfg := network.NewBondConfigV1Alpha1("bond0")
				cfg.BondLinks = []string{"eth0", "eth1"}

				return cfg
			},

			expectedError: "bond mode must be specified",
		},
		{
			name: "valid",
			cfg: func() *network.BondConfigV1Alpha1 {
				cfg := network.NewBondConfigV1Alpha1("bond25")
				cfg.BondLinks = []string{"eth0", "eth1"}
				cfg.BondMode = new(nethelpers.BondMode8023AD)
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

			expectedWarnings: []string{
				"miimon was not specified for 802.3ad bond",
				"updelay was not specified for 802.3ad bond",
				"downdelay was not specified for 802.3ad bond",
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
