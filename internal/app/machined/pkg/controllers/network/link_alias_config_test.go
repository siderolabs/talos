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

func (suite *LinkAliasConfigSuite) TestMachineConfigurationNewStyle() {
	lc1 := networkcfg.NewLinkAliasConfigV1Alpha1("net0")
	lc1.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`glob("00:1a:2b:*", mac(link.permanent_addr))`, celenv.LinkLocator()))

	lc2 := networkcfg.NewLinkAliasConfigV1Alpha1("net1")
	lc2.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`glob("33:44:55:*", mac(link.permanent_addr))`, celenv.LinkLocator()))

	ctr, err := container.New(lc1, lc2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	for _, link := range []struct {
		name          string
		permanentAddr string
	}{
		{
			name:          "enp0s2",
			permanentAddr: "00:1a:2b:33:44:55",
		},
		{
			name:          "enp1s3",
			permanentAddr: "33:44:55:66:77:88",
		},
		{
			name:          "enp1s4",
			permanentAddr: "33:44:55:66:77:89",
		},
	} {
		pAddr, err := net.ParseMAC(link.permanentAddr)
		suite.Require().NoError(err)

		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().PermanentAddr = nethelpers.HardwareAddr(pAddr)
		status.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(pAddr)
		status.TypedSpec().Type = nethelpers.LinkEther

		suite.Create(status)
	}

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "enp0s2", func(spec *network.LinkAliasSpec, asrt *assert.Assertions) {
		asrt.Equal("net0", spec.TypedSpec().Alias)
	})
	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp1s3")
	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp1s4")

	suite.Destroy(cfg)

	rtestutils.AssertNoResource[*network.LinkAliasSpec](suite.Ctx(), suite.T(), suite.State(), "enp0s2")
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
