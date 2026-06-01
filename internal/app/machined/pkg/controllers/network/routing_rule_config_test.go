// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RoutingRuleConfigSuite struct {
	ctest.DefaultSuite
}

//nolint:goconst
func (suite *RoutingRuleConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RoutingRuleConfigController{}))

	rc1 := networkcfg.NewRoutingRuleConfigV1Alpha1(1100)
	rc1.RuleSrc = networkcfg.Prefix{Prefix: netip.MustParsePrefix("10.0.0.0/8")}
	rc1.RuleTable = nethelpers.RoutingTable(100)
	rc1.RuleAction = nethelpers.RoutingRuleActionUnicast

	rc2 := networkcfg.NewRoutingRuleConfigV1Alpha1(1200)
	rc2.RuleDst = networkcfg.Prefix{Prefix: netip.MustParsePrefix("192.168.0.0/16")}
	rc2.RuleTable = nethelpers.RoutingTable(200)

	rc3 := networkcfg.NewRoutingRuleConfigV1Alpha1(1300)
	rc3.RuleSrc = networkcfg.Prefix{Prefix: netip.MustParsePrefix("2001:db8::/32")}
	rc3.RuleTable = nethelpers.RoutingTable(100)

	rc4 := networkcfg.NewRoutingRuleConfigV1Alpha1(1400)
	rc4.RuleTable = nethelpers.RoutingTable(100)

	ctr, err := container.New(rc1, rc2, rc3, rc4)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResources(
		suite,
		[]string{
			"configuration/inet4/01100",
			"configuration/inet4/01200",
			"configuration/inet6/01300",
			"configuration/inet4/01400",
			"configuration/inet6/01400",
		},
		func(r *network.RoutingRuleSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

			if strings.Contains(r.Metadata().ID(), "inet4") {
				asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
			} else {
				asrt.Equal(nethelpers.FamilyInet6, r.TypedSpec().Family)
			}

			switch r.Metadata().ID() {
			case "configuration/inet4/01100":
				asrt.Equal(netip.MustParsePrefix("10.0.0.0/8"), r.TypedSpec().Src)
				asrt.Equal(netip.Prefix{}, r.TypedSpec().Dst)
				asrt.Equal(nethelpers.RoutingTable(100), r.TypedSpec().Table)
				asrt.EqualValues(1100, r.TypedSpec().Priority)
				asrt.Equal(nethelpers.RoutingRuleActionUnicast, r.TypedSpec().Action)
			case "configuration/inet4/01200":
				asrt.Equal(netip.Prefix{}, r.TypedSpec().Src)
				asrt.Equal(netip.MustParsePrefix("192.168.0.0/16"), r.TypedSpec().Dst)
				asrt.Equal(nethelpers.RoutingTable(200), r.TypedSpec().Table)
				asrt.EqualValues(1200, r.TypedSpec().Priority)
				asrt.Equal(nethelpers.RoutingRuleActionUnicast, r.TypedSpec().Action) // defaults to unicast
			case "configuration/inet6/01300":
				asrt.Equal(netip.MustParsePrefix("2001:db8::/32"), r.TypedSpec().Src)
				asrt.Equal(netip.Prefix{}, r.TypedSpec().Dst)
				asrt.Equal(nethelpers.RoutingTable(100), r.TypedSpec().Table)
				asrt.EqualValues(1300, r.TypedSpec().Priority)
			case "configuration/inet4/01400":
				asrt.Equal(netip.Prefix{}, r.TypedSpec().Src)
				asrt.Equal(netip.Prefix{}, r.TypedSpec().Dst)
				asrt.Equal(nethelpers.RoutingTable(100), r.TypedSpec().Table)
				asrt.EqualValues(1400, r.TypedSpec().Priority)
			case "configuration/inet6/01400":
				asrt.Equal(netip.Prefix{}, r.TypedSpec().Src)
				asrt.Equal(netip.Prefix{}, r.TypedSpec().Dst)
				asrt.Equal(nethelpers.RoutingTable(100), r.TypedSpec().Table)
				asrt.EqualValues(1400, r.TypedSpec().Priority)
			}
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

//nolint:goconst
func (suite *RoutingRuleConfigSuite) TestMachineConfigurationWithFwMark() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RoutingRuleConfigController{}))

	rc1 := networkcfg.NewRoutingRuleConfigV1Alpha1(500)
	rc1.RuleTable = nethelpers.RoutingTable(100)
	rc1.RuleFwMark = 0x100
	rc1.RuleFwMask = 0xff00
	rc1.RuleIIFName = "eth0"
	rc1.RuleOIFName = "eth1"

	ctr, err := container.New(rc1)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResources(
		suite,
		[]string{
			"configuration/inet4/00500",
			"configuration/inet6/00500",
		},
		func(r *network.RoutingRuleSpec, asrt *assert.Assertions) {
			if strings.Contains(r.Metadata().ID(), "inet4") {
				asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
			} else {
				asrt.Equal(nethelpers.FamilyInet6, r.TypedSpec().Family)
			}

			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
			asrt.Equal(nethelpers.RoutingTable(100), r.TypedSpec().Table)
			asrt.EqualValues(500, r.TypedSpec().Priority)
			asrt.EqualValues(0x100, r.TypedSpec().FwMark)
			asrt.EqualValues(0xff00, r.TypedSpec().FwMask)
			asrt.Equal("eth0", r.TypedSpec().IIFName)
			asrt.Equal("eth1", r.TypedSpec().OIFName)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

// TestReservedPrioritySkipped verifies that user configs at kernel-reserved
// priorities never produce a RoutingRuleSpec, so the spec controller can
// never tear down (and thus delete) kernel-managed rules at those priorities.
func (suite *RoutingRuleConfigSuite) TestReservedPrioritySkipped() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RoutingRuleConfigController{}))

	// Validation rejects these priorities at apply time, but processConfig
	// must also defensively skip them to protect against any path that
	// bypasses validation.
	reserved := networkcfg.NewRoutingRuleConfigV1Alpha1(0)
	reserved.RuleTable = nethelpers.RoutingTable(100)
	reserved.RuleSrc = networkcfg.Prefix{Prefix: netip.MustParsePrefix("10.0.0.0/8")}
	reserved.RuleAction = nethelpers.RoutingRuleActionUnicast

	allowed := networkcfg.NewRoutingRuleConfigV1Alpha1(2000)
	allowed.RuleTable = nethelpers.RoutingTable(100)
	allowed.RuleAction = nethelpers.RoutingRuleActionUnicast

	ctr, err := container.New(reserved, allowed)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResources(
		suite,
		[]string{
			"configuration/inet4/02000",
			"configuration/inet6/02000",
		},
		func(*network.RoutingRuleSpec, *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	ctest.AssertNoResource[*network.RoutingRuleSpec](suite, "configuration/inet4/00000")
	ctest.AssertNoResource[*network.RoutingRuleSpec](suite, "configuration/inet6/00000")
}

func TestRoutingRuleConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RoutingRuleConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
