// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst
package network_test

import (
	"net"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type LinkAliasConfigSuite struct {
	ctest.DefaultSuite
}

type testLink struct {
	name          string
	permanentAddr string
}

func (suite *LinkAliasConfigSuite) createLinks(links []testLink) {
	for _, link := range links {
		pAddr, err := net.ParseMAC(link.permanentAddr)
		suite.Require().NoError(err)

		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().PermanentAddr = nethelpers.HardwareAddr(pAddr)
		status.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(pAddr)
		status.TypedSpec().Type = nethelpers.LinkEther

		suite.Create(status)
	}
}

func (suite *LinkAliasConfigSuite) TestMachineConfigurationNewStyle() {
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("net0")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`glob("00:1a:2b:*", mac(link.permanent_addr))`, celenv.LinkLocator()))

	lc2 := networkcfg.NewLinkAliasConfigV1Alpha1("net1")
	lc2.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`glob("33:44:55:*", mac(link.permanent_addr))`, celenv.LinkLocator()))

	ctr, err := container.New(lc1, lc2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.createLinks([]testLink{
		{name: "enp0s2", permanentAddr: "00:1a:2b:33:44:55"},
		{name: "enp1s3", permanentAddr: "33:44:55:66:77:88"},
		{name: "enp1s4", permanentAddr: "33:44:55:66:77:89"},
	})

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp1s3")
	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp1s4")

	suite.Destroy(cfg)

	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp0s2")
}

func (suite *LinkAliasConfigSuite) TestMachineConfigurationTwoAliasesSameLink() {
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("net1")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`glob("00:1a:2b:*", mac(link.permanent_addr))`, celenv.LinkLocator()))

	lc2 := networkcfg.NewLinkAliasConfigV1Alpha1("net0")
	lc2.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`glob("00:1a:2b:33:*", mac(link.permanent_addr))`, celenv.LinkLocator()))

	ctr, err := container.New(lc1, lc2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.createLinks([]testLink{
		{name: "enp0s2", permanentAddr: "00:1a:2b:33:44:55"},
	})

	// the "smallest" alias (net0) should win, net1 should be ignored since it conflicts with net0
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
}

func (suite *LinkAliasConfigSuite) TestPatternAliasSortsByMAC() {
	// Test that pattern aliases are assigned in alphabetical order, regardless of creation order
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("net%d")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`link.type == 1`, celenv.LinkLocator()))

	ctr, err := container.New(lc1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	// Create links out of order
	suite.createLinks([]testLink{
		{name: "enp1s4", permanentAddr: "33:44:55:66:77:88"},
		{name: "enp0s2", permanentAddr: "00:1a:2b:33:44:55"},
		{name: "enp1s3", permanentAddr: "33:44:55:66:77:89"},
	})

	// Aliases should follow alphabetical order of link name
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s3", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net2", spec.TypedSpec().Alias)
	})
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s4", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net1", spec.TypedSpec().Alias)
	})

	suite.Destroy(cfg)
}

func (suite *LinkAliasConfigSuite) TestPatternSkipsAlreadyAliased() {
	// Test that a fixed-name config claims a link, and a subsequent pattern config skips it
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("mgmt0")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`mac(link.permanent_addr) == "00:1a:2b:33:44:55"`, celenv.LinkLocator()))

	lc2 := networkcfg.NewLinkAliasConfigV1Alpha1("net%d")
	lc2.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`link.type == 1`, celenv.LinkLocator()))

	ctr, err := container.New(lc1, lc2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.createLinks([]testLink{
		{name: "enp0s2", permanentAddr: "00:1a:2b:33:44:55"},
		{name: "enp1s3", permanentAddr: "33:44:55:66:77:88"},
		{name: "enp1s4", permanentAddr: "33:44:55:66:77:89"},
	})

	// enp0s2 gets mgmt0 from the fixed-name config
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("mgmt0", spec.TypedSpec().Alias)
	})
	// enp1s3 and enp1s4 get net0 and net1 from the pattern config (enp0s2 skipped)
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s3", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s4", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net1", spec.TypedSpec().Alias)
	})

	suite.Destroy(cfg)
}

func (suite *LinkAliasConfigSuite) TestPatternReconcileOnLinkChange() {
	// Test that when links change, pattern aliases are reconciled (re-numbered)
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("net%d")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`link.type == 1`, celenv.LinkLocator()))

	ctr, err := container.New(lc1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.createLinks([]testLink{
		{name: "enp0s2", permanentAddr: "00:1a:2b:33:44:55"},
		{name: "enp1s3", permanentAddr: "33:44:55:66:77:88"},
		{name: "enp1s4", permanentAddr: "33:44:55:66:77:89"},
	})

	// Initial state: net0, net1, net2
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s3", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net1", spec.TypedSpec().Alias)
	})
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s4", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net2", spec.TypedSpec().Alias)
	})

	// Remove the middle link — aliases should be re-numbered
	suite.Destroy(network.NewLinkStatus(network.NamespaceName, "enp1s3"))

	// enp1s3 alias should be cleaned up, enp1s4 re-numbered to net1
	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp1s3")
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s4", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net1", spec.TypedSpec().Alias)
	})

	suite.Destroy(cfg)
}

func TestLinkAliasConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LinkAliasConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.LinkAliasConfigController{}))
			},
		},
	})
}
