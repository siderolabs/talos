// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RoutingRuleMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *RoutingRuleMergeSuite) assertRoutingRules(requiredIDs []string, check func(*network.RoutingRuleSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check)
}

func (suite *RoutingRuleMergeSuite) assertNoRoutingRule(id string) {
	ctest.AssertNoResource[*network.RoutingRuleSpec](suite, id)
}

func (suite *RoutingRuleMergeSuite) TestMerge() {
	// Create two rules with the same key (family/src/dst/priority) but different config layers.
	// The higher layer should win.
	cmdline := network.NewRoutingRuleSpec(network.ConfigNamespaceName, "cmdline/inet4/01000")
	*cmdline.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Src:         netip.MustParsePrefix("10.0.0.0/8"),
		Table:       nethelpers.RoutingTable(100),
		Priority:    1000,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigCmdline,
	}

	machineConfig := network.NewRoutingRuleSpec(network.ConfigNamespaceName, "configuration/inet4/01000")
	*machineConfig.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Src:         netip.MustParsePrefix("10.0.0.0/8"),
		Table:       nethelpers.RoutingTable(200),
		Priority:    1000,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	// A unique rule with no conflict.
	static := network.NewRoutingRuleSpec(network.ConfigNamespaceName, "configuration/inet4/02000")
	*static.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Dst:         netip.MustParsePrefix("192.168.0.0/16"),
		Table:       nethelpers.RoutingTable(123),
		Priority:    2000,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{cmdline, machineConfig, static} {
		suite.Create(res)
	}

	suite.assertRoutingRules(
		[]string{
			"inet4/01000",
			"inet4/02000",
		},
		func(r *network.RoutingRuleSpec, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())

			switch r.Metadata().ID() {
			case "inet4/01000":
				// machineConfig (ConfigMachineConfiguration) has higher layer than cmdline (ConfigCmdline)
				asrt.Equal(*machineConfig.TypedSpec(), *r.TypedSpec())
			case "inet4/02000":
				asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
			}
		},
	)

	// Remove the higher-layer resource; cmdline should now surface.
	suite.Destroy(machineConfig)

	suite.assertRoutingRules(
		[]string{
			"inet4/01000",
			"inet4/02000",
		},
		func(r *network.RoutingRuleSpec, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())

			switch r.Metadata().ID() {
			case "inet4/01000":
				asrt.Equal(*cmdline.TypedSpec(), *r.TypedSpec())
			case "inet4/02000":
				asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
			}
		},
	)

	// Destroy the static rule and verify it disappears.
	suite.Destroy(static)

	suite.assertNoRoutingRule("inet4/02000")
}

//nolint:gocyclo
func (suite *RoutingRuleMergeSuite) TestMergeFlapping() {
	// Simulate two conflicting rule definitions which are getting removed/added constantly.
	cmdline := network.NewRoutingRuleSpec(network.ConfigNamespaceName, "cmdline/inet4/00500")
	*cmdline.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Src:         netip.MustParsePrefix("10.0.0.0/8"),
		Table:       nethelpers.RoutingTable(100),
		Priority:    500,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigCmdline,
	}

	machineConfig := network.NewRoutingRuleSpec(network.ConfigNamespaceName, "configuration/inet4/00500")
	*machineConfig.TypedSpec() = network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Src:         netip.MustParsePrefix("10.0.0.0/8"),
		Table:       nethelpers.RoutingTable(200),
		Priority:    500,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	testMergeFlapping(&suite.DefaultSuite, []*network.RoutingRuleSpec{cmdline, machineConfig}, "inet4/00500", machineConfig)
}

func TestRoutingRuleMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RoutingRuleMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(netctrl.NewRoutingRuleMergeController()))
			},
		},
	})
}
