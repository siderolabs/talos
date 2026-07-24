// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// BGPSuite verifies native BGP against the embedded fabric peer started by `talosctl cluster create --with-bgp`.
type BGPSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *BGPSuite) SuiteName() string {
	return "api.BGPSuite"
}

// SetupTest ...
func (suite *BGPSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *BGPSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestNumberedBGP configures a node to peer with the embedded fabric peer, and verifies the session
// comes up, the node loopback is originated, and the fabric-advertised route is installed.
func (suite *BGPSuite) TestNumberedBGP() {
	if !suite.BGPEnabled {
		suite.T().Skip("skipping BGP test; enable with -talos.bgp (requires a cluster created with --with-bgp)")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping BGP test since provisioner is not qemu")
	}

	// the --with-bgp fabric peer runs on the bridge gateway and advertises this route (see initBGP).
	const (
		nodeASN     = 65001
		fabricASN   = 65000
		nodeLoopbk  = "10.99.0.10/32"
		fabricRoute = "10.200.0.0/24"
	)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	// the fabric peer is on the bridge gateway, derived from the node's subnet (not hard-coded, so the
	// test works for any --cidr).
	fabric := suite.bridgeGateway(nodeCtx)

	suite.T().Logf("testing native BGP on node %q against fabric peer %s", node, fabric)

	// carry the advertised loopback /32 on the always-present lo (the controller advertises it and filters
	// the 127/8 + ::1 loopback addresses) — no dummy interface needed.
	lo := network.NewLinkConfigV1Alpha1("lo")
	lo.LinkUp = new(true)
	lo.LinkAddresses = []network.AddressConfig{
		{AddressAddress: netip.MustParsePrefix(nodeLoopbk)},
	}

	bgp := network.NewBGPInstanceConfigV1Alpha1("fabric")
	bgp.BGPLocalASN = nodeASN
	bgp.BGPAdvertise = []string{"lo"}
	bgp.BGPNeighborConfigs = []network.BGPNeighborConfig{
		{
			NeighborAddressConfig: meta.Addr{Addr: fabric},
			NeighborPeerASN:       fabricASN,
		},
	}

	suite.PatchMachineConfig(nodeCtx, lo, bgp)

	// session should reach Established with the fabric peer.
	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, "fabric/"+fabric.String(),
		func(peer *networkres.BGPPeerStatus, asrt *assert.Assertions) {
			asrt.Equal("fabric", peer.TypedSpec().Instance)
			asrt.Equal(nethelpers.BGPSessionStateEstablished, peer.TypedSpec().State)
			asrt.Equal(uint32(fabricASN), peer.TypedSpec().PeerASN)
		},
	)

	// the route advertised by the fabric peer should be installed via the peering address.
	learnedRouteID := networkres.RouteID(
		nethelpers.TableMain,
		nethelpers.FamilyInet4,
		netip.MustParsePrefix(fabricRoute),
		fabric,
		0,
		"",
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, learnedRouteID,
		func(route *networkres.RouteStatus, asrt *assert.Assertions) {
			asrt.Equal(fabric, route.TypedSpec().Gateway)
			asrt.Equal(nethelpers.ProtocolBGP, route.TypedSpec().Protocol)
		},
	)

	// cleanup: removing the config should tear down the session and the learned route.
	suite.RemoveMachineConfigDocuments(nodeCtx, network.BGPInstanceKind)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.LinkKind, "lo")

	rtestutils.AssertNoResource[*networkres.BGPPeerStatus](nodeCtx, suite.T(), suite.Client.COSI, "fabric/"+fabric.String())
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, learnedRouteID)
}

// TestVRFBGP verifies that the embedded fabric peer can initiate a session into a passive BGP
// neighbor whose listener is bound to a Linux VRF, while a default-domain listener uses the same port.
func (suite *BGPSuite) TestVRFBGP() {
	if !suite.BGPEnabled {
		suite.T().Skip("skipping BGP test; enable with -talos.bgp (requires a cluster created with --with-bgp)")
	}

	if suite.BGPCLOSEnabled {
		suite.T().Skip("skipping numbered VRF BGP test on a full-CLOS cluster")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping BGP test since provisioner is not qemu")
	}

	const (
		nodeASN        = 65001
		fabricASN      = 65000
		nodeLoopback   = "10.99.0.11/32"
		fabricRoute    = "10.200.0.0/24"
		vrfName        = "vrf-bgp"
		defaultBGPName = "fabric"
		vrfBGPName     = "vrf-fabric"
	)

	const vrfTable = nethelpers.RoutingTable(100)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	managementLink, managementPrefix := suite.managementNetwork(nodeCtx)
	fabric := managementPrefix.Masked().Addr().Next()
	vrfAddress := vm.VRFPeerAddress()

	vrfLink := suite.vrfBGPLink(nodeCtx)

	suite.T().Logf(
		"testing inbound VRF BGP listener on node %q: management %s, VRF link %s, fabric %s -> %s",
		node,
		managementLink,
		vrfLink,
		fabric,
		vrfAddress,
	)

	lo := network.NewLinkConfigV1Alpha1("lo")
	lo.LinkUp = new(true)
	lo.LinkAddresses = []network.AddressConfig{{AddressAddress: netip.MustParsePrefix(nodeLoopback)}}

	vrfLinkConfig := network.NewLinkConfigV1Alpha1(vrfLink)
	vrfLinkConfig.LinkUp = new(true)
	vrfLinkConfig.LinkAddresses = []network.AddressConfig{{AddressAddress: vm.VRFPeerPrefix()}}
	vrfLinkConfig.LinkRoutes = []network.RouteConfig{{
		RouteDestination: meta.Prefix{Prefix: netip.PrefixFrom(fabric, fabric.BitLen())},
		RouteTable:       vrfTable,
	}}

	loopbackAddressID := "lo/" + nodeLoopback
	vrfAddressID := vrfLink + "/" + vm.VRFPeerPrefix().String()
	vrfFabricRouteID := networkres.RouteID(
		vrfTable,
		nethelpers.FamilyInet4,
		netip.PrefixFrom(fabric, fabric.BitLen()),
		netip.Addr{},
		networkres.DefaultRouteMetric,
		vrfLink,
	)

	vrf := network.NewVRFConfigV1Alpha1(vrfName)
	vrf.VRFLinks = []string{vrfLink}
	vrf.VRFTable = vrfTable
	vrf.LinkUp = new(true)

	defaultBGP := network.NewBGPInstanceConfigV1Alpha1(defaultBGPName)
	defaultBGP.BGPLocalASN = nodeASN
	defaultBGP.BGPAdvertise = []string{"lo"}
	defaultBGP.BGPNeighborConfigs = []network.BGPNeighborConfig{{
		NeighborAddressConfig: meta.Addr{Addr: fabric},
		NeighborPeerASN:       fabricASN,
	}}

	vrfBGP := network.NewBGPInstanceConfigV1Alpha1(vrfBGPName)
	vrfBGP.BGPVRF = vrfName
	vrfBGP.BGPLocalASN = nodeASN
	vrfBGP.BGPRouterID = meta.Addr{Addr: vrfAddress}
	vrfBGP.BGPNeighborConfigs = []network.BGPNeighborConfig{{
		NeighborAddressConfig: meta.Addr{Addr: fabric},
		NeighborPeerASN:       fabricASN,
		NeighborPassive:       true,
	}}

	suite.PatchMachineConfig(nodeCtx, lo, vrfLinkConfig, vrf, defaultBGP, vrfBGP)
	suite.waitForBGPPeer(nodeCtx, defaultBGPName+"/"+fabric.String(), defaultBGPName, fabricASN)

	vrfPeerID := vrfBGPName + "/" + fabric.String()
	suite.waitForBGPPeer(nodeCtx, vrfPeerID, vrfBGPName, fabricASN)

	learnedRouteID := networkres.RouteID(
		vrfTable,
		nethelpers.FamilyInet4,
		netip.MustParsePrefix(fabricRoute),
		fabric,
		0,
		"",
	)

	suite.waitForBGPRoute(nodeCtx, learnedRouteID, vrfTable, fabric)
	suite.waitForManagementNetwork(nodeCtx, managementLink, managementPrefix)

	finalManagementLink, finalManagementPrefix := suite.managementNetwork(nodeCtx)
	suite.Require().Equal(managementLink, finalManagementLink)
	suite.Require().Equal(managementPrefix, finalManagementPrefix)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.BGPInstanceKind, defaultBGPName, vrfBGPName)

	rtestutils.AssertNoResource[*networkres.BGPPeerStatus](nodeCtx, suite.T(), suite.Client.COSI, defaultBGPName+"/"+fabric.String())
	rtestutils.AssertNoResource[*networkres.BGPPeerStatus](nodeCtx, suite.T(), suite.Client.COSI, vrfPeerID)
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, learnedRouteID)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.VRFKind, vrfName)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.LinkKind, "lo", vrfLink)

	rtestutils.AssertNoResource[*networkres.AddressStatus](nodeCtx, suite.T(), suite.Client.COSI, loopbackAddressID)
	rtestutils.AssertNoResource[*networkres.AddressStatus](nodeCtx, suite.T(), suite.Client.COSI, vrfAddressID)
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, vrfFabricRouteID)
	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, vrfName)

	finalManagementLink, finalManagementPrefix = suite.managementNetwork(nodeCtx)
	suite.Require().Equal(managementLink, finalManagementLink)
	suite.Require().Equal(managementPrefix, finalManagementPrefix)
}

// assertBFDUp waits until BFD is up on every BGP peer.
func (suite *BGPSuite) assertBFDUp(nodeCtx context.Context) {
	rtestutils.AssertAll(
		nodeCtx, suite.T(), suite.Client.COSI,
		func(peer *networkres.BGPPeerStatus, asrt *assert.Assertions) {
			asrt.Equal("up", peer.TypedSpec().BFDState)
		},
	)
}

// assertSessionsEstablished waits until exactly count BGP peers are present and all Established.
func (suite *BGPSuite) assertSessionsEstablished(nodeCtx context.Context, count int) {
	rtestutils.AssertLength[*networkres.BGPPeerStatus](nodeCtx, suite.T(), suite.Client.COSI, count)
	rtestutils.AssertAll(
		nodeCtx, suite.T(), suite.Client.COSI,
		func(peer *networkres.BGPPeerStatus, asrt *assert.Assertions) {
			asrt.Equal(nethelpers.BGPSessionStateEstablished, peer.TypedSpec().State)
		},
	)
}

// assertLearnedLoopback waits until the destination is installed via the expected number of
// IPv6-link-local next-hops (RFC 8950), one per fabric link.
func (suite *BGPSuite) assertLearnedLoopback(nodeCtx context.Context, dest netip.Prefix, hops int) {
	suite.Eventually(func() bool {
		routes, err := safe.StateListAll[*networkres.RouteStatus](nodeCtx, suite.Client.COSI)
		require.NoError(suite.T(), err)

		for route := range routes.All() {
			spec := route.TypedSpec()

			if spec.Destination != dest || spec.Protocol != nethelpers.ProtocolBGP {
				continue
			}

			if hops == 1 {
				return spec.Gateway.IsLinkLocalUnicast() && spec.OutLinkName != ""
			}

			if len(spec.NextHops) != hops {
				return false
			}

			return slices.IndexFunc(spec.NextHops, func(nh networkres.RouteNextHop) bool {
				return !nh.Gateway.IsLinkLocalUnicast() || nh.OutLinkName == ""
			}) == -1
		}

		return false
	}, time.Minute, time.Second, "route to %s via %d link-local next-hop(s) not installed", dest, hops)
}

// TestBGPCLOS verifies a full-CLOS cluster (--with-bgp-clos): every node has NO management net0, only
// dedicated unnumbered fabric uplink(s) to the host BGP fabric peer and a loopback identity, reachable
// only via BGP. The per-node config (loopback on lo + unnumbered BGPInstanceConfig over the fabric uplinks) is
// baked at cluster-create time (a no-net0 node is unreachable until BGP is up), so this test only asserts
// the converged state. The fact talosctl can reach each node at its loopback is itself proof the host
// installed the BGP routes.
func (suite *BGPSuite) TestBGPCLOS() {
	if !suite.BGPCLOSEnabled {
		suite.T().Skip("skipping BGP CLOS test; enable with -talos.bgp.clos (requires a cluster created with --with-bgp-clos)")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping BGP CLOS test since provisioner is docker")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes, "no nodes discovered")

	suite.T().Logf("testing full-CLOS BGP across nodes %v (reachable only via their BGP loopbacks)", nodes)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)

		uplinks := suite.closFabricLinks(nodeCtx)
		suite.Require().NotEmpty(uplinks, "no fabric uplinks found on node %q", node)

		// one unnumbered session per uplink with the host fabric peer, all Established with BFD up.
		suite.assertSessionsEstablished(nodeCtx, len(uplinks))
		suite.assertBFDUp(nodeCtx)

		// the fabric peer advertises a default route over every uplink (ECMP when >1) — a no-net0 node
		// relies on it for everything off its loopback.
		suite.assertDefaultRoute(nodeCtx)

		// the fabric peer re-advertises every other node's loopback (eBGP next-hop-self), so each node
		// reaches the others over the fabric (ECMP across the uplinks when >1).
		for _, other := range nodes {
			if other == node {
				continue
			}

			suite.assertLearnedLoopback(nodeCtx, netip.MustParsePrefix(other+"/32"), len(uplinks))
		}

		// authentic CLOS edge: no net0/DHCP — the only routable address is the loopback on lo, every
		// physical interface is IPv6-link-local only.
		suite.assertNoManagementAddress(nodeCtx)
	}

	// anycast control-plane HA: every control-plane node advertises a shared k8s-API VIP /32, so the
	// fabric ECMPs across the CPs and a non-control-plane node reaches the API purely over BGP. (The
	// cluster having formed with its k8s endpoint = the VIP already proves the host reaches it.)
	cps := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)
	suite.Require().NotEmpty(cps, "no control-plane nodes discovered")

	vip := suite.discoverCLOSVIP(client.WithNode(suite.ctx, cps[0]), cps[0])
	suite.Require().True(vip.IsValid(), "no anycast k8s-API VIP found on control-plane %q", cps[0])

	suite.T().Logf("anycast k8s-API VIP is %s (advertised by %d control plane(s))", vip, len(cps))

	// every control-plane node carries the same VIP on lo.
	for _, cp := range cps[1:] {
		other := suite.discoverCLOSVIP(client.WithNode(suite.ctx, cp), cp)
		suite.Assert().Equal(vip, other, "control-plane %q advertises a different VIP", cp)
	}

	// a worker (non-CP) learns the VIP via BGP from the fabric (ECMP across its uplinks).
	workers := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	if len(workers) == 0 {
		suite.T().Log("no worker nodes; skipping cross-node VIP reachability assertion")

		return
	}

	workerCtx := client.WithNode(suite.ctx, workers[0])
	suite.assertLearnedLoopback(workerCtx, netip.PrefixFrom(vip, vip.BitLen()), len(suite.closFabricLinks(workerCtx)))
}

// discoverCLOSVIP returns the shared anycast k8s-API VIP carried on a control-plane node's lo: the global
// IPv4 address on lo that is not the node's own loopback identity (ownIP).
func (suite *BGPSuite) discoverCLOSVIP(nodeCtx context.Context, ownIP string) netip.Addr {
	own, err := netip.ParseAddr(ownIP)
	suite.Require().NoError(err)

	addresses, err := safe.StateListAll[*networkres.AddressStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	for address := range addresses.All() {
		spec := address.TypedSpec()
		if spec.LinkName != "lo" {
			continue
		}

		ip := spec.Address.Addr()
		if ip.Is4() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && ip != own {
			return ip
		}
	}

	return netip.Addr{}
}

// closFabricLinks returns the sorted names of a full-CLOS node's fabric uplinks: the virtio-net physical
// NICs (a no-net0 node has only these — there is no management bridge interface).
func (suite *BGPSuite) closFabricLinks(nodeCtx context.Context) []string {
	links, err := safe.StateListAll[*networkres.LinkStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var names []string

	for link := range links.All() {
		spec := link.TypedSpec()

		if spec.Physical() && spec.OperationalState == nethelpers.OperStateUp && spec.Driver == "virtio_net" {
			names = append(names, link.Metadata().ID())
		}
	}

	slices.Sort(names)

	return names
}

// managementNetwork returns the node's physical virtio-net management link and IPv4 subnet.
func (suite *BGPSuite) managementNetwork(nodeCtx context.Context) (string, netip.Prefix) {
	link, prefix, err := suite.lookupManagementNetwork(nodeCtx)
	suite.Require().NoError(err)

	return link, prefix
}

func (suite *BGPSuite) lookupManagementNetwork(nodeCtx context.Context) (string, netip.Prefix, error) {
	links, err := safe.StateListAll[*networkres.LinkStatus](nodeCtx, suite.Client.COSI)
	if err != nil {
		return "", netip.Prefix{}, err
	}

	var net0 string

	for link := range links.All() {
		if link.TypedSpec().Physical() && link.TypedSpec().Driver == "virtio_net" {
			net0 = link.Metadata().ID()

			break
		}
	}

	if net0 == "" {
		return "", netip.Prefix{}, fmt.Errorf("no virtio-net management interface found")
	}

	addresses, err := safe.StateListAll[*networkres.AddressStatus](nodeCtx, suite.Client.COSI)
	if err != nil {
		return "", netip.Prefix{}, err
	}

	for address := range addresses.All() {
		spec := address.TypedSpec()

		if spec.LinkName == net0 && spec.Address.Addr().Is4() {
			return net0, spec.Address.Masked(), nil
		}
	}

	return "", netip.Prefix{}, fmt.Errorf("no IPv4 management address on %q", net0)
}

func (suite *BGPSuite) waitForManagementNetwork(nodeCtx context.Context, expectedLink string, expectedPrefix netip.Prefix) {
	suite.Eventually(func() bool {
		link, prefix, err := suite.lookupManagementNetwork(nodeCtx)

		return err == nil && link == expectedLink && prefix == expectedPrefix
	}, 2*time.Minute, time.Second, "management network %s %s did not recover after VRF configuration", expectedLink, expectedPrefix)
}

func (suite *BGPSuite) waitForBGPPeer(nodeCtx context.Context, id, instance string, peerASN uint32) {
	suite.Eventually(func() bool {
		peer, err := safe.StateGetByID[*networkres.BGPPeerStatus](nodeCtx, suite.Client.COSI, id)
		if err != nil {
			return false
		}

		spec := peer.TypedSpec()

		return spec.Instance == instance && spec.State == nethelpers.BGPSessionStateEstablished && spec.PeerASN == peerASN
	}, 2*time.Minute, time.Second, "BGP peer %q did not reach Established", id)
}

func (suite *BGPSuite) waitForBGPRoute(
	nodeCtx context.Context,
	id string,
	table nethelpers.RoutingTable,
	gateway netip.Addr,
) {
	suite.Eventually(func() bool {
		route, err := safe.StateGetByID[*networkres.RouteStatus](nodeCtx, suite.Client.COSI, id)
		if err != nil {
			return false
		}

		spec := route.TypedSpec()

		return spec.Table == table && spec.Gateway == gateway && spec.Protocol == nethelpers.ProtocolBGP
	}, 2*time.Minute, time.Second, "BGP route %q was not installed", id)
}

// bridgeGateway returns the bridge gateway address (where the --with-bgp fabric peer listens): the first
// host IP of the node's net0 subnet, matching the provisioner's gateway allocation.
func (suite *BGPSuite) bridgeGateway(nodeCtx context.Context) netip.Addr {
	_, prefix := suite.managementNetwork(nodeCtx)

	return prefix.Masked().Addr().Next()
}

// vrfBGPLink returns the extra e1000 NIC attached to the management bridge for the inbound VRF test.
func (suite *BGPSuite) vrfBGPLink(nodeCtx context.Context) string {
	links, err := safe.StateListAll[*networkres.LinkStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var result []string

	for link := range links.All() {
		spec := link.TypedSpec()

		if spec.Physical() && spec.Driver == "e1000" {
			result = append(result, link.Metadata().ID())
		}
	}

	suite.Require().Len(result, 1, "expected exactly one e1000 VRF test NIC")

	return result[0]
}

// assertDefaultRoute waits until a BGP-learned default route is installed (the fabric peer's default).
func (suite *BGPSuite) assertDefaultRoute(nodeCtx context.Context) {
	suite.Eventually(func() bool {
		routes, err := safe.StateListAll[*networkres.RouteStatus](nodeCtx, suite.Client.COSI)
		require.NoError(suite.T(), err)

		for route := range routes.All() {
			spec := route.TypedSpec()

			// the default route is observed in RouteStatus with an empty destination (the zero
			// netip.Prefix, whose Bits() is -1), not as "0.0.0.0/0" — match either form.
			isDefault := !spec.Destination.IsValid() || spec.Destination.Bits() == 0

			if isDefault && spec.Protocol == nethelpers.ProtocolBGP {
				return true
			}
		}

		return false
	}, time.Minute, time.Second, "no BGP-learned default route installed")
}

// assertNoManagementAddress asserts a full-CLOS node has no net0/DHCP: its physical interfaces (the
// fabric uplinks) carry only IPv6 link-local addresses. The routable identity lives on lo, and the k8s
// CNI virtual interfaces (flannel.1, cni0, veth*) legitimately carry pod-CIDR addresses — both are
// non-physical and intentionally ignored.
func (suite *BGPSuite) assertNoManagementAddress(nodeCtx context.Context) {
	links, err := safe.StateListAll[*networkres.LinkStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	physical := map[string]struct{}{}

	for link := range links.All() {
		if link.TypedSpec().Physical() {
			physical[link.Metadata().ID()] = struct{}{}
		}
	}

	addresses, err := safe.StateListAll[*networkres.AddressStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	for address := range addresses.All() {
		spec := address.TypedSpec()

		if _, ok := physical[spec.LinkName]; !ok {
			continue
		}

		addr := spec.Address.Addr()

		suite.Assert().True(addr.IsLinkLocalUnicast(),
			"physical interface %q has a non-link-local address %s (expected no net0/DHCP on a full-CLOS node)", spec.LinkName, addr)
	}
}

// TestBGPCLOSFailover verifies BFD-driven failover and ECMP on a full-CLOS node: bringing one fabric
// uplink admin-down tears that session down (BFD detects the peer loss fast) and drops the corresponding
// ECMP next-hop, while the node stays reachable over the surviving uplink(s); restoring the uplink brings
// the session and the next-hop back. Requires >=2 uplinks per node.
func (suite *BGPSuite) TestBGPCLOSFailover() {
	if !suite.BGPCLOSEnabled {
		suite.T().Skip("skipping BGP CLOS failover test; enable with -talos.bgp.clos (requires a cluster created with --with-bgp-clos)")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping BGP CLOS failover test since provisioner is docker")
	}

	// use a worker so control-plane/etcd is undisturbed; it stays reachable via the surviving uplink.
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	uplinks := suite.closFabricLinks(nodeCtx)
	if len(uplinks) < 2 {
		suite.T().Skip("BGP CLOS failover test needs >=2 fabric uplinks per node")
	}

	// another node's loopback, learned via every uplink (ECMP) — watch it fail over.
	var target netip.Prefix

	for _, other := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		if other != node {
			target = netip.MustParsePrefix(other + "/32")

			break
		}
	}

	suite.Require().True(target.IsValid(), "no peer node to use as a failover target")

	suite.T().Logf("BGP CLOS failover on worker %q (uplinks %v), watching %s", node, uplinks, target)

	// baseline: one session per uplink (BFD up), target learned via every uplink (ECMP).
	suite.assertSessionsEstablished(nodeCtx, len(uplinks))
	suite.assertBFDUp(nodeCtx)
	suite.assertLearnedLoopback(nodeCtx, target, len(uplinks))

	// bring one uplink down: BFD detects the peer loss and tears that session down; the route fails over
	// to the surviving uplink(s).
	suite.setLinkUp(nodeCtx, uplinks[0], false)

	suite.assertSessionsEstablished(nodeCtx, len(uplinks)-1)
	suite.assertLearnedLoopback(nodeCtx, target, len(uplinks)-1)

	// restore the uplink: the session re-establishes (BFD up) and the ECMP next-hop returns.
	suite.setLinkUp(nodeCtx, uplinks[0], true)

	suite.assertSessionsEstablished(nodeCtx, len(uplinks))
	suite.assertBFDUp(nodeCtx)
	suite.assertLearnedLoopback(nodeCtx, target, len(uplinks))
}

// setLinkUp toggles the admin state of a fabric uplink via a merged LinkConfig patch.
func (suite *BGPSuite) setLinkUp(nodeCtx context.Context, iface string, up bool) {
	link := network.NewLinkConfigV1Alpha1(iface)
	link.LinkUp = new(up)

	suite.PatchMachineConfig(nodeCtx, link)
}

func init() {
	allSuites = append(allSuites, new(BGPSuite))
}
