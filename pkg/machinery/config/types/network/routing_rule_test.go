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

//go:embed testdata/routingruleconfig.yaml
var expectedRoutingRuleConfigDocument []byte

//nolint:goconst
func TestRoutingRuleConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewRoutingRuleConfigV1Alpha1(1000)
	cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cfg.RuleTable = nethelpers.RoutingTable(100)
	cfg.RuleAction = nethelpers.RoutingRuleActionUnicast

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedRoutingRuleConfigDocument, marshaled)
}

//nolint:goconst
func TestRoutingRuleConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedRoutingRuleConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.RoutingRuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.RoutingRuleKind,
		},
		RulePriority: "1000",
		RuleSrc:      network.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		RuleTable:    nethelpers.RoutingTable(100),
		RuleAction:   nethelpers.RoutingRuleActionUnicast,
	}, docs[0])
}

//nolint:goconst
func TestRoutingRuleConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.RoutingRuleConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				return &network.RoutingRuleConfigV1Alpha1{}
			},

			expectedError: "name must be specified\ninvalid name: must be priority parsable unsigned integer: strconv.ParseUint: parsing \"\": invalid syntax\npriority must be between 1 and 32765 (excluding reserved priorities [0 32500 32766 32767])\neither table or a non-unicast action must be specified", //nolint:lll
		},
		{
			name: "fwMask without fwMark",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(1000)
				cfg.RuleTable = nethelpers.RoutingTable(100)
				cfg.RuleFwMask = 0xff00

				return cfg
			},

			expectedError: "fwMask requires fwMark to be set",
		},
		{
			name: "missing name",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := &network.RoutingRuleConfigV1Alpha1{}
				cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
				cfg.RuleTable = nethelpers.RoutingTable(100)

				return cfg
			},

			expectedError: "name must be specified",
		},
		{
			name: "reserved priority 32766",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(32766)
				cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
				cfg.RuleTable = nethelpers.RoutingTable(100)

				return cfg
			},

			expectedError: "priority must be between 1 and 32765 (excluding reserved priorities [0 32500 32766 32767])",
		},
		{
			name: "reserved priority 32767",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(32767)
				cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
				cfg.RuleTable = nethelpers.RoutingTable(100)

				return cfg
			},

			expectedError: "priority must be between 1 and 32765 (excluding reserved priorities [0 32500 32766 32767])",
		},
		{
			name: "valid priority 1",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(1)
				cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
				cfg.RuleTable = nethelpers.RoutingTable(100)

				return cfg
			},
		},
		{
			name: "valid priority 32765",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(32765)
				cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
				cfg.RuleTable = nethelpers.RoutingTable(100)

				return cfg
			},
		},
		{
			name: "no table no action",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(1000)

				return cfg
			},

			expectedError: "either table or a non-unicast action must be specified",
		},
		{
			name: "valid with table only",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(1000)
				cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
				cfg.RuleTable = nethelpers.RoutingTable(100)

				return cfg
			},
		},
		{
			name: "valid with blackhole action",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(1000)
				cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
				cfg.RuleAction = nethelpers.RoutingRuleActionBlackhole

				return cfg
			},
		},
		{
			name: "valid with all fields",
			cfg: func() *network.RoutingRuleConfigV1Alpha1 {
				cfg := network.NewRoutingRuleConfigV1Alpha1(1000)
				cfg.RuleSrc = network.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
				cfg.RuleDst = network.Prefix{netip.MustParsePrefix("192.168.0.0/16")}
				cfg.RuleTable = nethelpers.RoutingTable(100)
				cfg.RuleAction = nethelpers.RoutingRuleActionUnicast
				cfg.RuleIIFName = "eth0"
				cfg.RuleOIFName = "eth1"
				cfg.RuleFwMark = 0x100
				cfg.RuleFwMask = 0xff00

				return cfg
			},
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
