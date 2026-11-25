// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type BGPControllerSuite struct {
	ctest.DefaultSuite
}

func (suite *BGPControllerSuite) TestIndependentInstanceLifecycle() {
	fabric := network.NewBGPInstanceConfig("fabric")
	*fabric.TypedSpec() = network.BGPInstanceConfigSpec{
		LocalASN:      65001,
		RouterID:      netip.MustParseAddr("10.0.0.1"),
		VRFTable:      nethelpers.TableMain,
		InstallRoutes: true,
		Neighbors: []network.BGPNeighborConfigSpec{
			{
				Address: netip.MustParseAddr("192.0.2.1"),
				PeerASN: 65002,
				Passive: true,
			},
		},
	}

	workload := network.NewBGPInstanceConfig("workload")
	*workload.TypedSpec() = network.BGPInstanceConfigSpec{
		LocalASN:      65003,
		RouterID:      netip.MustParseAddr("10.0.0.2"),
		VRFTable:      88,
		InstallRoutes: true,
		Neighbors: []network.BGPNeighborConfigSpec{
			{
				Address: netip.MustParseAddr("192.0.2.2"),
				PeerASN: 65004,
				Passive: true,
			},
		},
	}

	suite.Create(fabric)
	suite.Create(workload)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, assertions *assert.Assertions) {
		assertions.Equal("fabric", res.TypedSpec().Instance)
		assertions.Equal("192.0.2.1", res.TypedSpec().Peer)
		assertions.Equal(uint32(65001), res.TypedSpec().LocalASN)
	}, rtestutils.WithNamespace(network.NamespaceName))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "workload/192.0.2.2", func(res *network.BGPPeerStatus, assertions *assert.Assertions) {
		assertions.Equal("workload", res.TypedSpec().Instance)
		assertions.Equal(uint32(65003), res.TypedSpec().LocalASN)
	}, rtestutils.WithNamespace(network.NamespaceName))

	var fabricPeerCreated time.Time

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, _ *assert.Assertions) {
		fabricPeerCreated = res.Metadata().Created()
	}, rtestutils.WithNamespace(network.NamespaceName))

	currentFabric, err := suite.State().Get(suite.Ctx(), fabric.Metadata())
	suite.Require().NoError(err)

	updatedFabric := currentFabric.(*network.BGPInstanceConfig)
	updatedFabric.TypedSpec().ImportRoutes = []network.BGPImportRouteSpec{{
		BGPInstance: "workload",
		Prefixes:    []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")},
	}}
	suite.Update(updatedFabric)

	// Import policy is reconciled inside the running target server and must not restart fabric peers.
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, assertions *assert.Assertions) {
		assertions.Equal(fabricPeerCreated, res.Metadata().Created())
	}, rtestutils.WithNamespace(network.NamespaceName))

	suite.Destroy(workload)

	rtestutils.AssertNoResource[*network.BGPPeerStatus](
		suite.Ctx(),
		suite.T(),
		suite.State(),
		"workload/192.0.2.2",
		rtestutils.WithNamespace(network.NamespaceName),
	)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, assertions *assert.Assertions) {
		assertions.Equal("fabric", res.TypedSpec().Instance)
	}, rtestutils.WithNamespace(network.NamespaceName))

	suite.Destroy(fabric)
	rtestutils.AssertNoResource[*network.BGPPeerStatus](
		suite.Ctx(),
		suite.T(),
		suite.State(),
		"fabric/192.0.2.1",
		rtestutils.WithNamespace(network.NamespaceName),
	)
}

func (suite *BGPControllerSuite) TestRuntimeStatusResolutionAndServerRestart() {
	advertiseLink := network.NewLinkStatus(network.NamespaceName, "dummy0")
	advertiseLink.TypedSpec().Index = 1
	advertiseLink.TypedSpec().Alias = "node-ip"
	suite.Create(advertiseLink)

	routeSource := network.NewAddressStatus(network.NamespaceName, "dummy0/10.0.0.2/32")
	routeSource.TypedSpec().Address = netip.MustParsePrefix("10.0.0.2/32")
	routeSource.TypedSpec().LinkIndex = 1
	routeSource.TypedSpec().LinkName = "dummy0"
	suite.Create(routeSource)

	fabric := network.NewBGPInstanceConfig("fabric")
	*fabric.TypedSpec() = network.BGPInstanceConfigSpec{
		LocalASN:       65001,
		RouterID:       netip.MustParseAddr("10.0.0.1"),
		RouteSource:    netip.MustParseAddr("10.0.0.2"),
		AdvertiseLinks: []string{"node-ip"},
		VRFTable:       nethelpers.TableMain,
		InstallRoutes:  true,
		Neighbors: []network.BGPNeighborConfigSpec{
			{
				Address: netip.MustParseAddr("192.0.2.1"),
				PeerASN: 65002,
				Passive: true,
			},
		},
	}
	suite.Create(fabric)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, assertions *assert.Assertions) {
		assertions.Equal(uint32(65001), res.TypedSpec().LocalASN)
	}, rtestutils.WithNamespace(network.NamespaceName))

	suite.Destroy(routeSource)

	// Runtime status loss must not tear down the last-known-good instance or its outputs.
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, assertions *assert.Assertions) {
		assertions.Equal(uint32(65001), res.TypedSpec().LocalASN)
	}, rtestutils.WithNamespace(network.NamespaceName))

	routeSource = network.NewAddressStatus(network.NamespaceName, "dummy0/10.0.0.2/32")
	routeSource.TypedSpec().Address = netip.MustParsePrefix("10.0.0.2/32")
	routeSource.TypedSpec().LinkIndex = 1
	routeSource.TypedSpec().LinkName = "dummy0"
	suite.Create(routeSource)

	current, err := suite.State().Get(suite.Ctx(), fabric.Metadata())
	suite.Require().NoError(err)

	updated := current.(*network.BGPInstanceConfig)
	updated.TypedSpec().LocalASN = 65005
	suite.Update(updated)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, assertions *assert.Assertions) {
		assertions.Equal(uint32(65005), res.TypedSpec().LocalASN)
	}, rtestutils.WithNamespace(network.NamespaceName))

	suite.Destroy(updated)
	rtestutils.AssertNoResource[*network.BGPPeerStatus](
		suite.Ctx(),
		suite.T(),
		suite.State(),
		"fabric/192.0.2.1",
		rtestutils.WithNamespace(network.NamespaceName),
	)
}

func (suite *BGPControllerSuite) TestRejectedPeerDoesNotStopOtherPeers() {
	fabric := network.NewBGPInstanceConfig("fabric")
	*fabric.TypedSpec() = network.BGPInstanceConfigSpec{
		LocalASN:      65001,
		RouterID:      netip.MustParseAddr("10.0.0.1"),
		VRFTable:      nethelpers.TableMain,
		InstallRoutes: true,
		Neighbors: []network.BGPNeighborConfigSpec{
			{
				Address: netip.MustParseAddr("192.0.2.1"),
				PeerASN: 65002,
				Passive: true,
			},
			{
				// GoBGP rejects it: the zone names a link which does not exist
				Address: netip.MustParseAddr("fe80::1%bgpmissing0"),
				PeerASN: 65002,
				Passive: true,
			},
		},
	}

	workload := network.NewBGPInstanceConfig("workload")
	*workload.TypedSpec() = network.BGPInstanceConfigSpec{
		LocalASN:      65003,
		RouterID:      netip.MustParseAddr("10.0.0.2"),
		VRFTable:      88,
		InstallRoutes: true,
		Neighbors: []network.BGPNeighborConfigSpec{
			{
				Address: netip.MustParseAddr("192.0.2.2"),
				PeerASN: 65004,
				Passive: true,
			},
		},
	}

	suite.Create(fabric)
	suite.Create(workload)

	var created time.Time

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, _ *assert.Assertions) {
		created = res.Metadata().Created()
	}, rtestutils.WithNamespace(network.NamespaceName))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "workload/192.0.2.2", func(*network.BGPPeerStatus, *assert.Assertions) {}, rtestutils.WithNamespace(network.NamespaceName))

	rtestutils.AssertNoResource[*network.BGPPeerStatus](suite.Ctx(), suite.T(), suite.State(), "fabric/fe80::1%bgpmissing0", rtestutils.WithNamespace(network.NamespaceName))

	// a restarted controller would stop every server and recreate the statuses
	time.Sleep(time.Second)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(res *network.BGPPeerStatus, assertions *assert.Assertions) {
		assertions.Equal(created, res.Metadata().Created())
	}, rtestutils.WithNamespace(network.NamespaceName))
}

func TestBGPControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &BGPControllerSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			s.Require().NoError(s.Runtime().RegisterController(&netctrl.BGPController{ListenPort: -1}))
		},
	})
}

type BGPControllerRetrySuite struct {
	ctest.DefaultSuite

	listener net.Listener
}

func (suite *BGPControllerRetrySuite) TestFailedInstanceIsRetried() {
	fabric := network.NewBGPInstanceConfig("fabric")
	*fabric.TypedSpec() = network.BGPInstanceConfigSpec{
		LocalASN:      65001,
		RouterID:      netip.MustParseAddr("10.0.0.1"),
		VRFTable:      nethelpers.TableMain,
		InstallRoutes: true,
		Neighbors: []network.BGPNeighborConfigSpec{
			{
				Address: netip.MustParseAddr("192.0.2.1"),
				PeerASN: 65002,
				Passive: true,
			},
		},
	}

	suite.Create(fabric)

	// the server can not start while the listen port is taken, and unrelated wakeups (here input
	// events, on a busy host also neighbor table churn) keep reconciling the failed instance meanwhile
	for i := range 20 {
		suite.Create(network.NewLinkStatus(network.NamespaceName, fmt.Sprintf("churn%d", i)))

		time.Sleep(50 * time.Millisecond)
	}

	rtestutils.AssertNoResource[*network.BGPPeerStatus](suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", rtestutils.WithNamespace(network.NamespaceName))

	// freeing the port changes no controller input: only the retry starts the server
	suite.Require().NoError(suite.listener.Close())

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric/192.0.2.1", func(*network.BGPPeerStatus, *assert.Assertions) {}, rtestutils.WithNamespace(network.NamespaceName))
}

func TestBGPControllerRetrySuite(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)

	port := int32(listener.Addr().(*net.TCPAddr).Port)

	suite.Run(t, &BGPControllerRetrySuite{
		listener: listener,
		Timeout:  10 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			s.Require().NoError(s.Runtime().RegisterController(&netctrl.BGPController{ListenPort: port}))
		},
	})
}
