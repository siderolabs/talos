// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

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

func TestInstanceAuthPasswordListenerKey(t *testing.T) {
	t.Parallel()

	skipWithoutTCPMD5(t)

	instance := internalbgp.NewInstance()
	t.Cleanup(instance.Stop)

	port := freeBGPImportTestPort(t)
	config := network.BGPInstanceConfigSpec{LocalASN: 65001}
	peer := internalbgp.Peer{
		Address: "127.0.0.2",
		Config: network.BGPNeighborConfigSpec{
			Address:      netip.MustParseAddr("127.0.0.2"),
			PeerASN:      65002,
			Passive:      true,
			AuthPassword: "secret",
		},
	}

	ensure := func() {
		require.NoError(t, instance.EnsureServer(
			t.Context(),
			zap.NewNop(),
			config,
			netip.MustParseAddr("10.0.0.1"),
			0,
			[]internalbgp.Peer{peer},
			nil,
			port,
			func() {},
		))
	}

	// connect dials the listener from the peer address without a TCP MD5 signature.
	connect := func() error {
		dialer := net.Dialer{
			LocalAddr: &net.TCPAddr{IP: net.ParseIP(peer.Address)},
			Timeout:   time.Second,
		}

		conn, err := dialer.DialContext(t.Context(), "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
		if err != nil {
			return err
		}

		return conn.Close()
	}

	ensure()

	// The kernel silently drops an unsigned SYN for a peer with an MD5 key, so the dial has to time out
	// rather than fail for any other reason (connection refused, bind error).
	var netErr net.Error

	err := connect()
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout(), "unsigned SYN must be dropped, got %v", err)

	server := internalbgp.InstanceServerForTest(instance)

	peer.Config.AuthPassword = ""

	ensure()
	require.Same(t, server, internalbgp.InstanceServerForTest(instance), "removing the password must not restart GoBGP")
	require.Eventually(t, func() bool {
		return connect() == nil
	}, 5*time.Second, 100*time.Millisecond, "the listener key must be removed together with the password")
}

// skipWithoutTCPMD5 skips the test when the kernel is built without CONFIG_TCP_MD5SIG.
func skipWithoutTCPMD5(t *testing.T) {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	require.NoError(t, err)

	defer unix.Close(fd) //nolint:errcheck

	sig := unix.TCPMD5Sig{Keylen: 1}
	sig.Addr.Family = unix.AF_INET
	sig.Key[0] = 'x'

	if err = unix.SetsockoptTCPMD5Sig(fd, unix.IPPROTO_TCP, unix.TCP_MD5SIG, &sig); err != nil {
		t.Skipf("kernel does not support TCP MD5 signatures: %v", err)
	}
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
