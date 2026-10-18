// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net/netip"
	"testing"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestOriginatedPathWithdrawalPropagates(t *testing.T) {
	ctx := t.Context()
	fabricPort := freeBGPImportTestPort(t)
	originServer := startBGPImportTestServer(t, ctx, 65001, "192.0.2.1", -1, false)
	fabricServer := startBGPImportTestServer(t, ctx, 65000, "192.0.2.2", fabricPort, false)
	connectBGPImportTestFabric(t, ctx, originServer, fabricServer, fabricPort)

	instance := internalbgp.NewInstance()
	internalbgp.SetInstanceServerForTest(instance, originServer)

	prefixes := []netip.Prefix{
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("2001:db8:1234::/48"),
	}

	require.NoError(t, instance.ReconcileOriginated(prefixes))
	require.Eventually(t, func() bool {
		for _, prefix := range prefixes {
			if listBGPImportTestPath(t, fabricServer, prefix) == nil {
				return false
			}
		}

		return true
	}, 5*time.Second, 10*time.Millisecond, "originated paths were not advertised to the fabric peer")

	require.NoError(t, instance.ReconcileOriginated(nil))
	require.Eventually(t, func() bool {
		for _, prefix := range prefixes {
			if listBGPImportTestPath(t, fabricServer, prefix) != nil {
				return false
			}
		}

		return true
	}, 5*time.Second, 10*time.Millisecond, "originated paths were not withdrawn from the fabric peer")
}

func TestNewInstance(t *testing.T) {
	t.Parallel()

	instance := internalbgp.NewInstance()

	assert.False(t, instance.Running())
	assert.True(t, internalbgp.InstanceMapsInitializedForTest(instance))

	instance.Stop()

	assert.False(t, instance.Running())
	assert.True(t, internalbgp.InstanceMapsInitializedForTest(instance))
}

func TestInstanceServerLifecycleAndReconciliation(t *testing.T) {
	t.Parallel()

	instance := internalbgp.NewInstance()
	t.Cleanup(instance.Stop)

	config := network.BGPInstanceConfigSpec{
		LocalASN: 65001,
		Neighbors: []network.BGPNeighborConfigSpec{
			{Address: netip.MustParseAddr("192.0.2.2"), PeerASN: 65002, Passive: true},
		},
	}
	peer := internalbgp.Peer{Address: "192.0.2.2", Config: config.Neighbors[0]}
	peerIfaces := map[netip.Addr]string{netip.MustParseAddr("fe80::2"): "eth0"}

	require.NoError(t, instance.EnsureServer(
		t.Context(),
		zap.NewNop(),
		config,
		netip.MustParseAddr("10.0.0.1"),
		0,
		[]internalbgp.Peer{peer},
		peerIfaces,
		-1,
		func() {},
	))
	require.True(t, instance.Running())
	require.Len(t, internalbgp.InstancePeerKeysForTest(instance), 1)
	assert.Equal(t, peerIfaces, internalbgp.InstancePeerIfacesForTest(instance), "initial server creation must retain peer interfaces")

	server := internalbgp.InstanceServerForTest(instance)
	peerHash := internalbgp.InstancePeerKeysForTest(instance)[peer.Address]

	require.NoError(t, instance.EnsureServer(
		t.Context(),
		zap.NewNop(),
		config,
		netip.MustParseAddr("10.0.0.1"),
		0,
		[]internalbgp.Peer{peer},
		peerIfaces,
		-1,
		func() {},
	))
	assert.Same(t, server, internalbgp.InstanceServerForTest(instance), "an unchanged server key must not restart GoBGP")

	peer.Config.PeerASN = 65003
	require.NoError(t, instance.EnsureServer(
		t.Context(),
		zap.NewNop(),
		config,
		netip.MustParseAddr("10.0.0.1"),
		0,
		[]internalbgp.Peer{peer},
		peerIfaces,
		-1,
		func() {},
	))
	assert.NotEqual(t, peerHash, internalbgp.InstancePeerKeysForTest(instance)[peer.Address], "a changed peer must be replaced incrementally")
	assert.Same(t, server, internalbgp.InstanceServerForTest(instance), "a peer-only change must not restart GoBGP")

	require.NoError(t, instance.EnsureServer(
		t.Context(),
		zap.NewNop(),
		config,
		netip.MustParseAddr("10.0.0.1"),
		0,
		nil,
		peerIfaces,
		-1,
		func() {},
	))
	assert.Empty(t, internalbgp.InstancePeerKeysForTest(instance))

	stalePrefix := netip.MustParsePrefix("192.0.2.128/25")
	retainedPrefix := netip.MustParsePrefix("2001:db8:1234::/48")
	newPrefix := netip.MustParsePrefix("198.51.100.0/24")

	require.NoError(t, instance.ReconcileOriginated([]netip.Prefix{stalePrefix, retainedPrefix}))
	assert.True(t, internalbgp.InstanceOriginatedForTest(instance, stalePrefix))
	assert.True(t, internalbgp.InstanceOriginatedForTest(instance, retainedPrefix))
	require.NotNil(t, listBGPImportTestPath(t, server, stalePrefix))
	require.NotNil(t, listBGPImportTestPath(t, server, retainedPrefix))

	require.NoError(t, instance.ReconcileOriginated([]netip.Prefix{retainedPrefix, newPrefix}))
	assert.False(t, internalbgp.InstanceOriginatedForTest(instance, stalePrefix))
	assert.True(t, internalbgp.InstanceOriginatedForTest(instance, retainedPrefix))
	assert.True(t, internalbgp.InstanceOriginatedForTest(instance, newPrefix))
	assert.Nil(t, listBGPImportTestPath(t, server, stalePrefix))
	require.NotNil(t, listBGPImportTestPath(t, server, retainedPrefix))
	require.NotNil(t, listBGPImportTestPath(t, server, newPrefix))

	require.NoError(t, instance.ReconcileOriginated(nil))
	assert.False(t, internalbgp.InstanceOriginatedForTest(instance, retainedPrefix))
	assert.False(t, internalbgp.InstanceOriginatedForTest(instance, newPrefix))
	assert.Nil(t, listBGPImportTestPath(t, server, retainedPrefix))
	assert.Nil(t, listBGPImportTestPath(t, server, newPrefix))

	require.NoError(t, instance.EnsureServer(
		t.Context(),
		zap.NewNop(),
		config,
		netip.MustParseAddr("10.0.0.9"),
		0,
		nil,
		peerIfaces,
		-1,
		func() {},
	))
	assert.NotSame(t, server, internalbgp.InstanceServerForTest(instance), "a changed server key must restart GoBGP")
	assert.Equal(t, peerIfaces, internalbgp.InstancePeerIfacesForTest(instance), "server restart must retain peer interfaces")
}

func TestChangedPeerIsUpdatedInPlace(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	instancePort := freeBGPImportTestPort(t)

	instance := internalbgp.NewInstance()
	t.Cleanup(instance.Stop)

	peer := internalbgp.Peer{
		Address: "127.0.0.2",
		Config:  network.BGPNeighborConfigSpec{Address: netip.MustParseAddr("127.0.0.2"), PeerASN: 65000, Passive: true},
	}

	ensure := func() {
		t.Helper()

		require.NoError(t, instance.EnsureServer(
			ctx,
			zap.NewNop(),
			network.BGPInstanceConfigSpec{LocalASN: 65001, Neighbors: []network.BGPNeighborConfigSpec{peer.Config}},
			netip.MustParseAddr("10.0.0.1"),
			0,
			[]internalbgp.Peer{peer},
			nil,
			instancePort,
			func() {},
		))
	}

	ensure()

	fabric := startBGPImportTestServer(t, ctx, 65000, "10.0.0.2", -1, false)
	require.NoError(t, fabric.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: &gobgpapi.Peer{
		Conf:      &gobgpapi.PeerConf{NeighborAddress: "127.0.0.1", PeerAsn: 65001},
		Transport: &gobgpapi.Transport{LocalAddress: "127.0.0.2", RemotePort: uint32(instancePort)},
	}}))

	peerState := func() *gobgpapi.Peer {
		t.Helper()

		var result *gobgpapi.Peer

		require.NoError(t, internalbgp.InstanceServerForTest(instance).ListPeer(ctx, &gobgpapi.ListPeerRequest{Address: peer.Address}, func(p *gobgpapi.Peer) {
			result = p
		}))
		require.NotNil(t, result)

		return result
	}

	require.Eventually(t, func() bool {
		return peerState().GetState().GetSessionState() == gobgpapi.PeerState_SESSION_STATE_ESTABLISHED
	}, 5*time.Second, 10*time.Millisecond, "BGP session did not establish")

	uptime := peerState().GetTimers().GetState().GetUptime().AsTime()

	// BFD and hold time changes must not reset the session
	peer.Config.HoldTime = 30 * time.Second
	peer.Config.BFD = &network.BGPBFDConfigSpec{TransmitInterval: 300 * time.Millisecond, ReceiveInterval: 300 * time.Millisecond, DetectMultiplier: 3}

	ensure()

	assert.Equal(t, internalbgp.PeerKey(peer), internalbgp.InstancePeerKeysForTest(instance)[peer.Address])

	updated := peerState()
	assert.EqualValues(t, 30, updated.GetTimers().GetConfig().GetHoldTime())
	assert.True(t, updated.GetBfd().GetEnabled())

	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		state := peerState()

		require.Equal(t, gobgpapi.PeerState_SESSION_STATE_ESTABLISHED, state.GetState().GetSessionState(), "BGP session must stay up through an in-place update")
		require.Equal(t, uptime, state.GetTimers().GetState().GetUptime().AsTime(), "BGP session must not be reset by an in-place update")
	}
}
