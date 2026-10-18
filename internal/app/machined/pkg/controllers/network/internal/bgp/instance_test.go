// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

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

func TestChangedPeersAreReplacedOneAtATime(t *testing.T) {
	t.Parallel()

	instance := internalbgp.NewInstance()
	t.Cleanup(instance.Stop)

	var established, synced map[string]bool

	internalbgp.SetPeerStatesForTest(instance, func(context.Context) (map[string]bool, map[string]bool) {
		return established, synced
	})

	signals := make(chan struct{}, 100)

	peerA := internalbgp.Peer{Address: "192.0.2.2", Config: network.BGPNeighborConfigSpec{Address: netip.MustParseAddr("192.0.2.2"), PeerASN: 65002, Passive: true}}
	peerB := internalbgp.Peer{Address: "192.0.2.3", Config: network.BGPNeighborConfigSpec{Address: netip.MustParseAddr("192.0.2.3"), PeerASN: 65003, Passive: true}}

	ensure := func() map[string]string {
		t.Helper()

		require.NoError(t, instance.EnsureServer(
			t.Context(),
			zap.NewNop(),
			network.BGPInstanceConfigSpec{LocalASN: 65001, Neighbors: []network.BGPNeighborConfigSpec{peerA.Config, peerB.Config}},
			netip.MustParseAddr("10.0.0.1"),
			0,
			[]internalbgp.Peer{peerA, peerB},
			nil,
			-1,
			func() { signals <- struct{}{} },
		))

		return internalbgp.InstancePeerKeysForTest(instance)
	}

	up := func(peers ...string) map[string]bool {
		m := map[string]bool{}

		for _, peer := range peers {
			m[peer] = true
		}

		return m
	}

	initial := ensure()

	// a change to both established peers in one apply replaces only the first one
	established, synced = up(peerA.Address, peerB.Address), up(peerA.Address, peerB.Address)
	peerA.Config.HoldTime, peerB.Config.HoldTime = 30*time.Second, 30*time.Second

	current := ensure()
	assert.NotEqual(t, initial[peerA.Address], current[peerA.Address], "the first changed peer must be replaced")
	assert.Equal(t, initial[peerB.Address], current[peerB.Address], "the second changed peer must wait")

	// the replaced peer is down, then established without End-of-RIB: the other one keeps waiting
	established, synced = up(peerB.Address), up(peerB.Address)
	assert.Equal(t, initial[peerB.Address], ensure()[peerB.Address])

	established = up(peerA.Address, peerB.Address)
	assert.Equal(t, initial[peerB.Address], ensure()[peerB.Address])

	// the replaced peer is back: the other one is replaced
	synced = up(peerA.Address, peerB.Address)
	assert.NotEqual(t, initial[peerB.Address], ensure()[peerB.Address], "the second changed peer must be replaced once the first one is back")

	// the replaced peer never comes back: the other one is replaced after the timeout, which also triggers a reconcile
	internalbgp.SetPeerReplaceTimeoutForTest(instance, 200*time.Millisecond)

	peerA.Config.HoldTime, peerB.Config.HoldTime = 60*time.Second, 60*time.Second
	before := ensure()
	established, synced = up(peerB.Address), up(peerB.Address)

	time.Sleep(100 * time.Millisecond)

	for len(signals) > 0 {
		<-signals
	}

	select {
	case <-signals:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "no reconcile was triggered after the timeout")
	}

	assert.NotEqual(t, before[peerB.Address], ensure()[peerB.Address], "the second changed peer must be replaced after the timeout")

	// changed peers that are down are replaced right away
	established, synced = nil, nil
	peerA.Config.HoldTime, peerB.Config.HoldTime = 90*time.Second, 90*time.Second
	current = ensure()
	assert.Equal(t, internalbgp.PeerKey(peerA), current[peerA.Address])
	assert.Equal(t, internalbgp.PeerKey(peerB), current[peerB.Address])
}
