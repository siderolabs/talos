// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"context"
	"maps"
	"net/netip"
	"slices"
	"testing"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestDynamicNeighborLifecycle(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	instancePort := freeBGPImportTestPort(t)

	instance := internalbgp.NewInstance()
	t.Cleanup(instance.Stop)

	prefix := netip.MustParsePrefix("127.0.0.0/30")
	neighbor := network.BGPNeighborConfigSpec{Prefix: prefix, PeerASN: 65000}

	ensure := func(neighbors ...network.BGPNeighborConfigSpec) {
		t.Helper()

		var resolved []internalbgp.Peer

		for _, n := range neighbors {
			if peer, ok := internalbgp.ResolvePeer(n, nil, nil, zap.NewNop()); ok {
				resolved = append(resolved, peer)
			}
		}

		require.NoError(t, instance.EnsureServer(
			ctx,
			zap.NewNop(),
			network.BGPInstanceConfigSpec{LocalASN: 65001, Neighbors: neighbors},
			[]netip.Addr{netip.MustParseAddr("10.0.0.1")},
			0,
			resolved,
			nil,
			instancePort,
			func() {},
		))
	}

	ensure(neighbor)

	dialBGPRemotePeers(t, ctx, instancePort, 1, "127.0.0.2", "127.0.0.3", "127.0.0.5")

	peers := func() map[string]*gobgpapi.Peer {
		t.Helper()

		result := map[string]*gobgpapi.Peer{}

		require.NoError(t, internalbgp.InstanceServerForTest(instance).ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(p *gobgpapi.Peer) {
			result[internalbgp.PeerStatus(p, 65001).Peer] = p
		}))

		return result
	}

	established := func(holdTime uint64) bool {
		current := peers()

		for _, address := range []string{"127.0.0.2", "127.0.0.3"} {
			peer, ok := current[address]
			if !ok || internalbgp.PeerStatus(peer, 65001).State != nethelpers.BGPSessionStateEstablished {
				return false
			}

			if peer.GetTimers().GetConfig().GetHoldTime() != holdTime {
				return false
			}
		}

		return true
	}

	require.Eventually(t, func() bool { return established(90) }, 10*time.Second, 10*time.Millisecond, "dynamic neighbors did not establish")

	assert.ElementsMatch(t, []string{"127.0.0.2", "127.0.0.3"}, slices.Collect(maps.Keys(peers())), "only peers within the prefix must be accepted")

	neighbor.HoldTime = 30 * time.Second

	ensure(neighbor)

	assert.Equal(t,
		map[netip.Prefix]string{prefix: internalbgp.PeerKey(internalbgp.DynamicNeighborPeer(neighbor, ""))},
		internalbgp.InstanceDynamicKeysForTest(instance),
	)
	require.Eventually(t, func() bool { return established(30) }, 10*time.Second, 10*time.Millisecond, "dynamic neighbors did not re-establish with the changed settings")

	ensure(neighbor, network.BGPNeighborConfigSpec{Address: netip.MustParseAddr("127.0.0.2"), PeerASN: 65000, Passive: true})

	require.Eventually(t, func() bool {
		peer, ok := peers()["127.0.0.2"]

		return ok && peer.GetConf().GetNeighborAddress() == "127.0.0.2" &&
			internalbgp.PeerStatus(peer, 65001).State == nethelpers.BGPSessionStateEstablished
	}, 10*time.Second, 10*time.Millisecond, "configured neighbor did not replace the dynamic one")

	ensure()

	assert.Empty(t, internalbgp.InstanceDynamicKeysForTest(instance))
	assert.Empty(t, peers())

	var groups []string

	require.NoError(t, internalbgp.InstanceServerForTest(instance).ListPeerGroup(ctx, &gobgpapi.ListPeerGroupRequest{}, func(group *gobgpapi.PeerGroup) {
		groups = append(groups, group.GetConf().GetPeerGroupName())
	}))
	assert.Empty(t, groups)
}

func TestPeerStatusAdvertisedCount(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	instancePort := freeBGPImportTestPort(t)

	instance := internalbgp.NewInstance()
	t.Cleanup(instance.Stop)

	neighbors := []network.BGPNeighborConfigSpec{
		{Prefix: netip.MustParsePrefix("127.0.0.8/30"), PeerASN: 65000},
		{Address: netip.MustParseAddr("127.0.0.12"), PeerASN: 65000, Passive: true},
	}

	var resolved []internalbgp.Peer

	for _, n := range neighbors {
		if peer, ok := internalbgp.ResolvePeer(n, nil, nil, zap.NewNop()); ok {
			resolved = append(resolved, peer)
		}
	}

	require.NoError(t, instance.EnsureServer(
		ctx,
		zap.NewNop(),
		network.BGPInstanceConfigSpec{LocalASN: 65001, Neighbors: neighbors},
		[]netip.Addr{netip.MustParseAddr("10.0.0.1")},
		0,
		resolved,
		nil,
		instancePort,
		func() {},
	))

	dialBGPRemotePeers(t, ctx, instancePort, 2, "127.0.0.9", "127.0.0.12")

	advertised := func(want uint32) bool {
		statuses := map[string]network.BGPPeerStatusSpec{}

		for _, status := range instance.Snapshot(ctx).PeerStatuses {
			statuses[status.Peer] = status
		}

		for _, address := range []string{"127.0.0.9", "127.0.0.12"} {
			status, ok := statuses[address]
			if !ok || status.State != nethelpers.BGPSessionStateEstablished || status.Advertised != want {
				return false
			}
		}

		return true
	}

	require.Eventually(t, func() bool { return advertised(0) }, 10*time.Second, 10*time.Millisecond, "peers did not establish")

	require.NoError(t, instance.ReconcileOriginated([]netip.Prefix{
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
	}))

	require.Eventually(t, func() bool { return advertised(2) }, 10*time.Second, 10*time.Millisecond, "originated prefixes are not counted as advertised")

	require.NoError(t, instance.ReconcileOriginated([]netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}))

	require.Eventually(t, func() bool { return advertised(1) }, 10*time.Second, 10*time.Millisecond, "withdrawn prefix is still counted as advertised")
}

func TestDerivedRouterIDIsKeptWhileDynamicNeighborIsEstablished(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	instancePort := freeBGPImportTestPort(t)

	instance := internalbgp.NewInstance()
	t.Cleanup(instance.Stop)

	neighbor := network.BGPNeighborConfigSpec{Prefix: netip.MustParsePrefix("127.0.0.16/30"), PeerASN: 65000}

	ensure := func(candidates []netip.Addr, neighbors ...network.BGPNeighborConfigSpec) {
		t.Helper()

		require.NoError(t, instance.EnsureServer(
			ctx,
			zap.NewNop(),
			network.BGPInstanceConfigSpec{LocalASN: 65001, Neighbors: neighbors},
			candidates,
			0,
			nil,
			nil,
			instancePort,
			func() {},
		))
	}

	routerID := func() string {
		t.Helper()

		bgp, err := internalbgp.InstanceServerForTest(instance).GetBgp(ctx, &gobgpapi.GetBgpRequest{})
		require.NoError(t, err)

		return bgp.GetGlobal().GetRouterId()
	}

	addrs := func(addrs ...string) []netip.Addr { return xslices.Map(addrs, netip.MustParseAddr) }

	ensure(addrs("10.5.0.3"), neighbor)

	dialBGPRemotePeers(t, ctx, instancePort, 3, "127.0.0.17")

	require.Eventually(t, func() bool {
		var established bool

		require.NoError(t, internalbgp.InstanceServerForTest(instance).ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(p *gobgpapi.Peer) {
			established = established || p.GetState().GetSessionState() == gobgpapi.PeerState_SESSION_STATE_ESTABLISHED
		}))

		return established
	}, 10*time.Second, 10*time.Millisecond, "dynamic neighbor did not establish")

	server := internalbgp.InstanceServerForTest(instance)

	ensure(addrs("10.5.0.2", "10.5.0.3"), neighbor)

	assert.Same(t, server, internalbgp.InstanceServerForTest(instance), "server rebuilt while a dynamic neighbor is established")
	assert.Equal(t, "10.5.0.3", routerID())

	ensure(addrs("10.5.0.2", "10.5.0.3"))

	assert.Equal(t, "10.5.0.2", routerID(), "router ID kept after the prefix neighbor was removed")
}

func dialBGPRemotePeers(t *testing.T, ctx context.Context, instancePort int32, routerIDOctet byte, locals ...string) {
	t.Helper()

	for i, local := range locals {
		remote := startBGPImportTestServer(t, ctx, 65000, netip.AddrFrom4([4]byte{10, 0, routerIDOctet, byte(i + 1)}).String(), -1, false)

		require.NoError(t, remote.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: &gobgpapi.Peer{
			Conf:      &gobgpapi.PeerConf{NeighborAddress: "127.0.0.1", PeerAsn: 65001},
			Transport: &gobgpapi.Transport{LocalAddress: local, RemotePort: uint32(instancePort)},
			Timers:    &gobgpapi.Timers{Config: &gobgpapi.TimersConfig{ConnectRetry: 1, IdleHoldTimeAfterReset: 1}},
		}}))
	}
}
