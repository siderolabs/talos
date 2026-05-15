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

//go:embed testdata/natruleconfig.yaml
var expectedNATRuleConfigDocument []byte

//go:embed testdata/natruleconfig_snat.yaml
var expectedNATRuleConfigSNATDocument []byte

//go:embed testdata/natruleconfig_dnat.yaml
var expectedNATRuleConfigDNATDocument []byte

func TestNATRuleConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewNATRuleConfigV1Alpha1()
	cfg.MetaName = "test"
	cfg.SourceAddress = network.NATSubnetConfig{
		IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("10.244.0.0/16")},
	}
	cfg.OutputInterface = network.NATInterfaceConfig{
		InterfaceNames: []string{"eth0"},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedNATRuleConfigDocument, marshaled)
}

func TestNATRuleConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedNATRuleConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.NATRuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.NATRuleConfigKind,
		},
		MetaName: "test",
		SourceAddress: network.NATSubnetConfig{
			IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("10.244.0.0/16")},
		},
		OutputInterface: network.NATInterfaceConfig{
			InterfaceNames: []string{"eth0"},
		},
	}, docs[0])
}

func TestNATRuleConfigSNATMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewNATRuleConfigV1Alpha1()
	cfg.MetaName = "test-snat"
	cfg.Type = nethelpers.NATTypeSNAT
	cfg.SourceAddress = network.NATSubnetConfig{
		IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
	}
	cfg.OutputInterface = network.NATInterfaceConfig{
		InterfaceNames: []string{"eth0"},
	}
	cfg.SNATAddr = network.Addr{Addr: netip.MustParseAddr("203.0.113.1")}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedNATRuleConfigSNATDocument, marshaled)
}

func TestNATRuleConfigSNATUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedNATRuleConfigSNATDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.NATRuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.NATRuleConfigKind,
		},
		MetaName: "test-snat",
		Type:     nethelpers.NATTypeSNAT,
		SourceAddress: network.NATSubnetConfig{
			IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		},
		OutputInterface: network.NATInterfaceConfig{
			InterfaceNames: []string{"eth0"},
		},
		SNATAddr: network.Addr{Addr: netip.MustParseAddr("203.0.113.1")},
	}, docs[0])
}

func TestNATRuleConfigDNATMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewNATRuleConfigV1Alpha1()
	cfg.MetaName = "test-dnat"
	cfg.Type = nethelpers.NATTypeDNAT
	cfg.InputInterface = network.NATInterfaceConfig{
		InterfaceNames: []string{"eth0"},
	}
	cfg.DestinationAddress = network.NATSubnetConfig{
		IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("203.0.113.1/32")},
	}
	cfg.DNATAddr = network.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	cfg.DNATPortNum = 8080

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedNATRuleConfigDNATDocument, marshaled)
}

func TestNATRuleConfigDNATUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedNATRuleConfigDNATDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.NATRuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.NATRuleConfigKind,
		},
		MetaName: "test-dnat",
		Type:     nethelpers.NATTypeDNAT,
		InputInterface: network.NATInterfaceConfig{
			InterfaceNames: []string{"eth0"},
		},
		DestinationAddress: network.NATSubnetConfig{
			IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("203.0.113.1/32")},
		},
		DNATAddr:    network.Addr{Addr: netip.MustParseAddr("10.0.0.1")},
		DNATPortNum: 8080,
	}, docs[0])
}

func TestNATRuleConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.NATRuleConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "missing name",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				return &network.NATRuleConfigV1Alpha1{}
			},
			expectedError: "name is required",
		},
		{
			name: "valid masquerade minimal",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "masq"

				return cfg
			},
		},
		{
			name: "valid masquerade with source and iface",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "masq"
				cfg.SourceAddress = network.NATSubnetConfig{
					IncludeSubnets: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
				}
				cfg.OutputInterface = network.NATInterfaceConfig{InterfaceNames: []string{"eth0"}}

				return cfg
			},
		},
		{
			name: "masquerade with snatAddress warns",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "masq"
				cfg.SNATAddr = network.Addr{Addr: netip.MustParseAddr("1.2.3.4")}

				return cfg
			},
			expectedWarnings: []string{"snatAddress has no effect on masquerade rules"},
		},
		{
			name: "masquerade with inputInterface warns",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "masq"
				cfg.InputInterface = network.NATInterfaceConfig{InterfaceNames: []string{"eth0"}}

				return cfg
			},
			expectedWarnings: []string{"inputInterface has no effect on masquerade rules (ingress interface is not matched at postrouting)"},
		},
		{
			name: "valid snat",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "snat"
				cfg.Type = nethelpers.NATTypeSNAT
				cfg.SNATAddr = network.Addr{Addr: netip.MustParseAddr("203.0.113.1")}

				return cfg
			},
		},
		{
			name: "snat missing snatAddress",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "snat"
				cfg.Type = nethelpers.NATTypeSNAT

				return cfg
			},
			expectedError: "snatAddress is required for type snat",
		},
		{
			name: "snat with inputInterface warns",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "snat"
				cfg.Type = nethelpers.NATTypeSNAT
				cfg.SNATAddr = network.Addr{Addr: netip.MustParseAddr("203.0.113.1")}
				cfg.InputInterface = network.NATInterfaceConfig{InterfaceNames: []string{"eth0"}}

				return cfg
			},
			expectedWarnings: []string{"inputInterface has no effect on snat rules (ingress interface is not matched at postrouting)"},
		},
		{
			name: "snat with dnatAddress warns",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "snat"
				cfg.Type = nethelpers.NATTypeSNAT
				cfg.SNATAddr = network.Addr{Addr: netip.MustParseAddr("203.0.113.1")}
				cfg.DNATAddr = network.Addr{Addr: netip.MustParseAddr("10.0.0.1")}

				return cfg
			},
			expectedWarnings: []string{"dnatAddress has no effect on snat rules"},
		},
		{
			name: "valid dnat",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "dnat"
				cfg.Type = nethelpers.NATTypeDNAT
				cfg.DNATAddr = network.Addr{Addr: netip.MustParseAddr("10.0.0.1")}

				return cfg
			},
		},
		{
			name: "valid dnat with port",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "dnat"
				cfg.Type = nethelpers.NATTypeDNAT
				cfg.DNATAddr = network.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
				cfg.DNATPortNum = 8080

				return cfg
			},
		},
		{
			name: "dnat missing dnatAddress",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "dnat"
				cfg.Type = nethelpers.NATTypeDNAT

				return cfg
			},
			expectedError: "dnatAddress is required for type dnat",
		},
		{
			name: "dnat with outputInterface warns",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "dnat"
				cfg.Type = nethelpers.NATTypeDNAT
				cfg.DNATAddr = network.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
				cfg.OutputInterface = network.NATInterfaceConfig{InterfaceNames: []string{"eth0"}}

				return cfg
			},
			expectedWarnings: []string{"outputInterface has no effect on dnat rules (egress interface is not known at prerouting)"},
		},
		{
			name: "dnat with snatAddress warns",
			cfg: func() *network.NATRuleConfigV1Alpha1 {
				cfg := network.NewNATRuleConfigV1Alpha1()
				cfg.MetaName = "dnat"
				cfg.Type = nethelpers.NATTypeDNAT
				cfg.DNATAddr = network.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
				cfg.SNATAddr = network.Addr{Addr: netip.MustParseAddr("203.0.113.1")}

				return cfg
			},
			expectedWarnings: []string{"snatAddress has no effect on dnat rules"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
