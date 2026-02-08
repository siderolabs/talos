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
	"github.com/siderolabs/go-pointer"
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

func (suite *LinkAliasConfigSuite) TestRequireUniqueMatchFalse() {
	// Test that when requireUniqueMatch is false, the first matching link is used
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("net0")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`glob("33:44:55:*", mac(link.permanent_addr))`, celenv.LinkLocator()))
	lc1.Selector.RequireUniqueMatch = pointer.To(false) // Allow multiple matches, use first

	ctr, err := container.New(lc1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.createLinks([]testLink{
		{name: "enp1s3", permanentAddr: "33:44:55:66:77:88"},
		{name: "enp1s4", permanentAddr: "33:44:55:66:77:89"},
	})

	// First link (enp1s3) should get the alias since it's first in iteration order
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s3", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp1s4")

	suite.Destroy(cfg)
}

func (suite *LinkAliasConfigSuite) TestSkipAliasedLinks() {
	// Test that skipAliasedLinks allows creating net0 and net1 from any two links
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("net0")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`link.type == 1`, celenv.LinkLocator())) // Match all ethernet links
	lc1.Selector.RequireUniqueMatch = pointer.To(false)
	lc1.Selector.SkipAliasedLinks = pointer.To(false) // First config doesn't need to skip

	lc2 := networkcfg.NewLinkAliasConfigV1Alpha1("net1")
	lc2.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`link.type == 1`, celenv.LinkLocator())) // Same selector
	lc2.Selector.RequireUniqueMatch = pointer.To(false)
	lc2.Selector.SkipAliasedLinks = pointer.To(true) // Skip links already aliased

	ctr, err := container.New(lc1, lc2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.createLinks([]testLink{
		{name: "enp0s2", permanentAddr: "00:1a:2b:33:44:55"},
		{name: "enp1s3", permanentAddr: "33:44:55:66:77:88"},
		{name: "enp1s4", permanentAddr: "33:44:55:66:77:89"},
	})

	// First link gets net0, second link gets net1
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s3", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net1", spec.TypedSpec().Alias)
	})
	// Third link doesn't get an alias
	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp1s4")

	suite.Destroy(cfg)
}

func (suite *LinkAliasConfigSuite) TestSkipAliasedLinksWithUniqueMatch() {
	// Test requireUniqueMatch=true (default) + skipAliasedLinks=true
	// First config matches exactly one link, second config skips the aliased link and matches exactly one remaining link
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("net0")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`mac(link.permanent_addr) == "00:1a:2b:33:44:55"`, celenv.LinkLocator()))
	// requireUniqueMatch defaults to true

	lc2 := networkcfg.NewLinkAliasConfigV1Alpha1("net1")
	lc2.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`glob("*", mac(link.permanent_addr))`, celenv.LinkLocator())) // Matches all links
	// requireUniqueMatch defaults to true
	lc2.Selector.SkipAliasedLinks = pointer.To(true) // Skip enp0s2 which got net0

	ctr, err := container.New(lc1, lc2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.createLinks([]testLink{
		{name: "enp0s2", permanentAddr: "00:1a:2b:33:44:55"},
		{name: "enp1s3", permanentAddr: "33:44:55:66:77:88"},
	})

	// First link gets net0 (exact match)
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	// Second link gets net1 (enp0s2 skipped due to skipAliasedLinks, leaving only enp1s3 as unique match)
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp1s3", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
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
