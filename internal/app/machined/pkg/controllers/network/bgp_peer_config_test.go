// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type BGPPeerConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *BGPPeerConfigSuite) TestRenderAndResolveLinkAliases() {
	advertiseLink := network.NewLinkStatus(network.NamespaceName, "dummy0")
	advertiseLink.TypedSpec().Alias = "node-ip"
	suite.Create(advertiseLink)

	peerLink := network.NewLinkStatus(network.NamespaceName, "eth0")
	peerLink.TypedSpec().AltNames = []string{"fabric0"}
	suite.Create(peerLink)

	cfg := networkcfg.NewBGPPeerConfigV1Alpha1()
	cfg.BGPLocalASN = 65001
	cfg.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	cfg.BGPRouteSource = meta.Addr{Addr: netip.MustParseAddr("10.0.0.2")}
	cfg.BGPAdvertise = []string{"node-ip"}
	cfg.BGPMultipath = true
	cfg.BGPMaxPaths = 4
	cfg.BGPNeighborConfigs = []networkcfg.BGPNeighborConfig{
		{
			NeighborLinkConfig: "fabric0",
			NeighborPeerASN:    65002,
			NeighborHoldTime:   9 * time.Second,
			NeighborBFDConfig: &networkcfg.BGPBFDConfig{
				BFDTransmitInterval: 300 * time.Millisecond,
				BFDReceiveInterval:  400 * time.Millisecond,
				BFDDetectMultiplier: 3,
			},
		},
		{
			NeighborAddressConfig: meta.Addr{Addr: netip.MustParseAddr("192.0.2.1")},
			NeighborPeerASN:       65003,
		},
	}

	ctr, err := container.New(cfg)
	suite.Require().NoError(err)

	machineConfig := configresource.NewMachineConfig(ctr)
	suite.Create(machineConfig)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), network.BGPPeerConfigID, func(res *network.BGPPeerConfig, assertions *assert.Assertions) {
		spec := res.TypedSpec()
		assertions.Equal(uint32(65001), spec.LocalASN)
		assertions.Equal(netip.MustParseAddr("10.0.0.1"), spec.RouterID)
		assertions.Equal(netip.MustParseAddr("10.0.0.2"), spec.RouteSource)
		assertions.Equal([]string{"dummy0"}, spec.AdvertiseLinks)
		assertions.True(spec.Multipath)
		assertions.Equal(uint8(4), spec.MaxPaths)

		if assertions.Len(spec.Neighbors, 2) {
			assertions.Equal("eth0", spec.Neighbors[0].Link)
			assertions.Equal(uint32(65002), spec.Neighbors[0].PeerASN)
			assertions.Equal(9*time.Second, spec.Neighbors[0].HoldTime)

			if assertions.NotNil(spec.Neighbors[0].BFD) {
				assertions.Equal(300*time.Millisecond, spec.Neighbors[0].BFD.TransmitInterval)
				assertions.Equal(400*time.Millisecond, spec.Neighbors[0].BFD.ReceiveInterval)
				assertions.Equal(uint8(3), spec.Neighbors[0].BFD.DetectMultiplier)
			}

			assertions.Equal(netip.MustParseAddr("192.0.2.1"), spec.Neighbors[1].Address)
			assertions.Empty(spec.Neighbors[1].Link)
		}
	}, rtestutils.WithNamespace(network.NamespaceName))

	suite.Destroy(machineConfig)

	rtestutils.AssertNoResource[*network.BGPPeerConfig](
		suite.Ctx(),
		suite.T(),
		suite.State(),
		network.BGPPeerConfigID,
		rtestutils.WithNamespace(network.NamespaceName),
	)
}

func (suite *BGPPeerConfigSuite) TestResolveAliasesWhenLinksAppear() {
	cfg := networkcfg.NewBGPPeerConfigV1Alpha1()
	cfg.BGPLocalASN = 65001
	cfg.BGPAdvertise = []string{"node-ip"}
	cfg.BGPNeighborConfigs = []networkcfg.BGPNeighborConfig{{NeighborLinkConfig: "fabric0"}}

	ctr, err := container.New(cfg)
	suite.Require().NoError(err)
	suite.Create(configresource.NewMachineConfig(ctr))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), network.BGPPeerConfigID, func(res *network.BGPPeerConfig, assertions *assert.Assertions) {
		assertions.Equal([]string{"node-ip"}, res.TypedSpec().AdvertiseLinks)
		assertions.Equal("fabric0", res.TypedSpec().Neighbors[0].Link)
	}, rtestutils.WithNamespace(network.NamespaceName))

	advertiseLink := network.NewLinkStatus(network.NamespaceName, "dummy0")
	advertiseLink.TypedSpec().Alias = "node-ip"
	suite.Create(advertiseLink)

	peerLink := network.NewLinkStatus(network.NamespaceName, "eth0")
	peerLink.TypedSpec().AltNames = []string{"fabric0"}
	suite.Create(peerLink)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), network.BGPPeerConfigID, func(res *network.BGPPeerConfig, assertions *assert.Assertions) {
		assertions.Equal([]string{"dummy0"}, res.TypedSpec().AdvertiseLinks)
		assertions.Equal("eth0", res.TypedSpec().Neighbors[0].Link)
	}, rtestutils.WithNamespace(network.NamespaceName))
}

func (suite *BGPPeerConfigSuite) TestNoConfig() {
	rtestutils.AssertNoResource[*network.BGPPeerConfig](
		suite.Ctx(),
		suite.T(),
		suite.State(),
		network.BGPPeerConfigID,
		rtestutils.WithNamespace(network.NamespaceName),
	)
}

func TestBGPPeerConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &BGPPeerConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.BGPPeerConfigController{}))
			},
		},
	})
}
