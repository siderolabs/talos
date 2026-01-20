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

//go:embed testdata/bondconfig.yaml
var expectedBondConfigDocument []byte

func TestBondConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewBondConfigV1Alpha1("agg.0")
	cfg.BondLinks = []string{"eth0", "eth1"}
	cfg.BondMode = pointer.To(nethelpers.BondMode8023AD)
	cfg.BondXmitHashPolicy = pointer.To(nethelpers.BondXmitPolicyLayer34)
	cfg.BondFailOverMAC = pointer.To(nethelpers.FailOverMACFollow)
	cfg.BondLACPRate = pointer.To(nethelpers.LACPRateSlow)
	cfg.BondMIIMon = pointer.To(uint32(100))
	cfg.BondUpDelay = pointer.To(uint32(200))
	cfg.BondDownDelay = pointer.To(uint32(200))
	cfg.BondResendIGMP = pointer.To(uint32(1))
	cfg.BondPacketsPerSlave = pointer.To(uint32(1))
	cfg.BondADActorSysPrio = pointer.To(uint16(65535))
	cfg.LinkUp = pointer.To(true)
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
		BondMode:            pointer.To(nethelpers.BondMode8023AD),
		BondXmitHashPolicy:  pointer.To(nethelpers.BondXmitPolicyLayer34),
		BondFailOverMAC:     pointer.To(nethelpers.FailOverMACFollow),
		BondLACPRate:        pointer.To(nethelpers.LACPRateSlow),
		BondMIIMon:          pointer.To(uint32(100)),
		BondUpDelay:         pointer.To(uint32(200)),
		BondDownDelay:       pointer.To(uint32(200)),
		BondResendIGMP:      pointer.To(uint32(1)),
		BondPacketsPerSlave: pointer.To(uint32(1)),
		BondADActorSysPrio:  pointer.To(uint16(65535)),
		CommonLinkConfig: network.CommonLinkConfig{
			LinkUp: pointer.To(true),
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
				cfg.BondMode = pointer.To(nethelpers.BondModeActiveBackup)

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
				cfg.BondMode = pointer.To(nethelpers.BondMode8023AD)
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
