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

//go:embed testdata/ruleconfig.yaml
var expectedRuleConfigDocument []byte

func TestRuleConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewRuleConfigV1Alpha1()
	cfg.MetaName = "test"

	cfg.PortSelector = network.RulePortSelector{
		Protocol: nethelpers.ProtocolUDP,
		Ports: network.PortRanges{
			{Lo: 53, Hi: 53},
			{Lo: 8000, Hi: 9000},
		},
	}

	cfg.Ingress = network.IngressConfig{
		{
			Subnet: netip.MustParsePrefix("192.168.0.0/16"),
			Except: network.Prefix{netip.MustParsePrefix("192.168.0.3/32")},
		},
		{
			Subnet: netip.MustParsePrefix("2001::/16"),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedRuleConfigDocument, marshaled)
}

func TestRuleConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedRuleConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.RuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.RuleConfigKind,
		},
		MetaName: "test",
		PortSelector: network.RulePortSelector{
			Protocol: nethelpers.ProtocolUDP,
			Ports: network.PortRanges{
				{Lo: 53, Hi: 53},
				{Lo: 8000, Hi: 9000},
			},
		},
		Ingress: network.IngressConfig{
			{
				Subnet: netip.MustParsePrefix("192.168.0.0/16"),
				Except: network.Prefix{netip.MustParsePrefix("192.168.0.3/32")},
			},
			{
				Subnet: netip.MustParsePrefix("2001::/16"),
			},
		},
	}, docs[0])
}

func TestRuleConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.RuleConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  network.NewRuleConfigV1Alpha1,

			expectedError: "name is required",
		},
		{
			name: "no ports",
			cfg: func() *network.RuleConfigV1Alpha1 {
				cfg := network.NewRuleConfigV1Alpha1()
				cfg.MetaName = "-"

				return cfg
			},

			expectedError: "portSelector.ports is required",
		},
		{
			name: "invalid port range",
			cfg: func() *network.RuleConfigV1Alpha1 {
				cfg := network.NewRuleConfigV1Alpha1()
				cfg.MetaName = "-"
				cfg.PortSelector.Ports = network.PortRanges{
					{Lo: 80, Hi: 80},
					{Lo: 80, Hi: 79},
				}

				return cfg
			},

			expectedError: "invalid port range: 80-79",
		},
		{
			name: "invalid subnet",
			cfg: func() *network.RuleConfigV1Alpha1 {
				cfg := network.NewRuleConfigV1Alpha1()
				cfg.MetaName = "--"
				cfg.PortSelector.Ports = network.PortRanges{
					{Lo: 80, Hi: 80},
				}
				cfg.Ingress = network.IngressConfig{
					{},
				}

				return cfg
			},

			expectedError: "invalid subnet: invalid Prefix",
		},
		{
			name: "valid",
			cfg: func() *network.RuleConfigV1Alpha1 {
				cfg := network.NewRuleConfigV1Alpha1()
				cfg.MetaName = "--"
				cfg.PortSelector.Ports = network.PortRanges{
					{Lo: 80, Hi: 80},
					{Lo: 6443, Hi: 6444},
				}
				cfg.Ingress = network.IngressConfig{
					{
						Subnet: netip.MustParsePrefix("192.168.0.0/16"),
						Except: network.Prefix{netip.MustParsePrefix("192.168.3.0/24")},
					},
					{
						Subnet: netip.MustParsePrefix("2001::/16"),
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

type validationMode struct{}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}

func (validationMode) InContainer() bool {
	return false
}
