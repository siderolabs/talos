// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/internal/pkg/testport"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

var testRouterID = []netip.Addr{netip.MustParseAddr("10.0.0.1")}

func TestOriginatedPaths(t *testing.T) {
	t.Parallel()

	instance := newTestInstance(t)
	instance.ensure(t, network.BGPInstanceConfigSpec{LocalASN: 65001}, testRouterID, testPeer("127.0.0.2", 65000))

	fabric := instance.startSpeaker(t, "127.0.0.2", 65000, 65001)
	instance.waitState(t, "127.0.0.2", nethelpers.BGPSessionStateEstablished)

	stale := netip.MustParsePrefix("192.0.2.128/25")
	retained := netip.MustParsePrefix("2001:db8:1234::/48")
	added := netip.MustParsePrefix("198.51.100.0/24")

	advertised := func(expected ...netip.Prefix) func() bool {
		return func() bool {
			for _, prefix := range []netip.Prefix{stale, retained, added} {
				if (listBGPImportTestPath(t, fabric, prefix) != nil) != slices.Contains(expected, prefix) {
					return false
				}
			}

			return true
		}
	}

	require.NoError(t, instance.ReconcileOriginated([]netip.Prefix{stale, retained}))
	require.Eventually(t, advertised(stale, retained), 5*time.Second, 10*time.Millisecond, "originated paths were not advertised")

	require.NoError(t, instance.ReconcileOriginated([]netip.Prefix{retained, added}))
	require.Eventually(t, advertised(retained, added), 5*time.Second, 10*time.Millisecond, "a stale originated path was not withdrawn")

	require.NoError(t, instance.ReconcileOriginated(nil))
	require.Eventually(t, advertised(), 5*time.Second, 10*time.Millisecond, "originated paths were not withdrawn")
}

func TestNewInstance(t *testing.T) {
	t.Parallel()

	instance := internalbgp.NewInstance()

	assert.False(t, instance.Running())

	instance.Stop()

	assert.False(t, instance.Running())
}

func TestInstanceAuthPasswordListenerKey(t *testing.T) {
	t.Parallel()

	skipWithoutTCPMD5(t)

	instance := newTestInstance(t)

	config := network.BGPInstanceConfigSpec{LocalASN: 65001}
	peer := testPeer("127.0.0.2", 65002)
	peer.Config.AuthPassword = "secret"

	// connect dials the listener from the peer address, signing the connection with the password if set.
	connect := func(password string) error {
		dialer := net.Dialer{
			LocalAddr: &net.TCPAddr{IP: net.ParseIP(peer.Address)},
			Timeout:   time.Second,
		}

		if password != "" {
			dialer.Control = func(_, _ string, conn syscall.RawConn) error {
				var sockoptErr error

				if err := conn.Control(func(fd uintptr) {
					sockoptErr = unix.SetsockoptTCPMD5Sig(int(fd), unix.IPPROTO_TCP, unix.TCP_MD5SIG, tcpMD5Sig(net.IPv4(127, 0, 0, 1), password))
				}); err != nil {
					return err
				}

				return sockoptErr
			}
		}

		conn, err := dialer.DialContext(t.Context(), "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(instance.port))))
		if err != nil {
			return err
		}

		return conn.Close()
	}

	// The kernel silently drops a SYN with a missing or mismatched MD5 signature for a peer with a key, so the
	// dial has to time out rather than fail for any other reason (connection refused, bind error).
	requireDropped := func(password string) {
		var netErr net.Error

		err := connect(password)
		require.ErrorAs(t, err, &netErr)
		require.True(t, netErr.Timeout(), "SYN with password %q must be dropped, got %v", password, err)
	}

	instance.ensure(t, config, testRouterID, peer)

	requireDropped("")
	require.NoError(t, connect("secret"))

	// Rotating the password must replace the listener key in place.
	peer.Config.AuthPassword = "rotated"

	instance.ensure(t, config, testRouterID, peer)
	require.Equal(t, 1, instance.starts(), "rotating the password must not restart GoBGP")
	require.Eventually(t, func() bool {
		return connect("rotated") == nil
	}, 5*time.Second, 100*time.Millisecond, "the listener key must be replaced together with the password")
	requireDropped("secret")
	requireDropped("")

	peer.Config.AuthPassword = ""

	instance.ensure(t, config, testRouterID, peer)
	require.Equal(t, 1, instance.starts(), "removing the password must not restart GoBGP")
	require.Eventually(t, func() bool {
		return connect("") == nil
	}, 5*time.Second, 100*time.Millisecond, "the listener key must be removed together with the password")
}

// tcpMD5Sig builds the TCP_MD5SIG option for an IPv4 peer.
func tcpMD5Sig(addr net.IP, password string) *unix.TCPMD5Sig {
	sig := &unix.TCPMD5Sig{Keylen: uint16(len(password))}
	sig.Addr.Family = unix.AF_INET
	copy(sig.Addr.Data[2:6], addr.To4())
	copy(sig.Key[:], password)

	return sig
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

	instance := newTestInstance(t)

	config := network.BGPInstanceConfigSpec{LocalASN: 65001}
	peer := testPeer("192.0.2.2", 65002)

	instance.ensure(t, config, testRouterID, peer)
	require.True(t, instance.Running())
	assert.Equal(t, uint32(65002), instance.peerStatus(t, peer.Address).PeerASN)

	instance.ensure(t, config, testRouterID, peer)
	assert.Equal(t, 1, instance.starts(), "an unchanged configuration must not restart GoBGP")

	peer.Config.PeerASN = 65003

	instance.ensure(t, config, testRouterID, peer)
	assert.Equal(t, uint32(65003), instance.peerStatus(t, peer.Address).PeerASN, "a changed peer must be updated")
	assert.Equal(t, 1, instance.starts(), "a peer-only change must not restart GoBGP")

	instance.ensure(t, config, testRouterID)
	assert.Empty(t, instance.Snapshot(t.Context()).PeerStatuses, "a removed peer must be deleted")

	instance.ensure(t, config, []netip.Addr{netip.MustParseAddr("10.0.0.9")})
	assert.Equal(t, 2, instance.starts(), "a changed router ID must restart GoBGP")
	assert.Equal(t, "10.0.0.9", instance.routerID(t))

	// the listener is bound to the VRF device: a VRF recreated under the same name gets a new index
	require.NoError(t, instance.EnsureServer(t.Context(), instance.logger, config, []netip.Addr{netip.MustParseAddr("10.0.0.9")}, 7, nil, nil, instance.port, func() {}))
	assert.Equal(t, 3, instance.starts(), "a changed VRF index must restart GoBGP")

	instance.Stop()
	assert.False(t, instance.Running())
}

// TestLinkLocalNexthopAfterRestart asserts that a link-local next-hop from an unnumbered peer still resolves to
// the peer interface after GoBGP is rebuilt: without it the route is dropped, see LearnedNexthop.
func TestLinkLocalNexthopAfterRestart(t *testing.T) {
	t.Parallel()

	instance := newTestInstance(t)
	instance.peerIfaces = map[netip.Addr]string{netip.MustParseAddr("127.0.0.2"): "lo"}

	peer := testPeer("127.0.0.2", 65001)
	prefix := netip.MustParsePrefix("2001:db8:1::/64")
	expected := []network.RouteNextHop{{Gateway: netip.MustParseAddr("fe80::2"), OutLinkName: "lo"}}

	// iBGP keeps the advertised next-hop
	speaker := instance.startSpeaker(t, peer.Address, 65001, 65001)

	nlri, err := bgppacket.NewIPAddrPrefix(prefix)
	require.NoError(t, err)

	mpReach, err := bgppacket.NewPathAttributeMpReachNLRI(bgppacket.RF_IPv6_UC, []bgppacket.PathNLRI{{NLRI: nlri}}, netip.MustParseAddr("fe80::2"))
	require.NoError(t, err)

	responses, err := speaker.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{{
		Family: bgppacket.RF_IPv6_UC,
		Nlri:   nlri,
		Attrs:  []bgppacket.PathAttributeInterface{bgppacket.NewPathAttributeOrigin(0), mpReach},
	}}})
	require.NoError(t, err)
	require.Len(t, responses, 1)
	require.NoError(t, responses[0].Error)

	// a configured router ID change restarts GoBGP even with the session established
	for i, routerID := range []string{"10.0.0.1", "10.0.0.9"} {
		config := network.BGPInstanceConfigSpec{LocalASN: 65001, RouterID: netip.MustParseAddr(routerID), InstallRoutes: true}

		instance.ensure(t, config, nil, peer)
		instance.SetOutputState(nil, config)

		assert.EventuallyWithT(t, func(collect *assert.CollectT) {
			assert.Equal(collect, expected, instance.Snapshot(t.Context()).Learned[prefix])
		}, 15*time.Second, 10*time.Millisecond, "a link-local next-hop must resolve to the peer interface after %d restarts", i)
	}

	assert.Equal(t, 2, instance.starts())
}

func TestChangedPeerIsUpdatedInPlace(t *testing.T) {
	t.Parallel()

	instance := newTestInstance(t)

	config := network.BGPInstanceConfigSpec{LocalASN: 65001}
	peer := testPeer("127.0.0.2", 65000)

	instance.ensure(t, config, testRouterID, peer)
	speaker := instance.startSpeaker(t, peer.Address, 65000, 65001)
	instance.waitState(t, peer.Address, nethelpers.BGPSessionStateEstablished)

	since := instance.peerStatus(t, peer.Address).Since

	// BFD and hold time changes must not reset the session
	peer.Config.HoldTime = 30 * time.Second
	peer.Config.BFD = &network.BGPBFDConfigSpec{TransmitInterval: 300 * time.Millisecond, ReceiveInterval: 300 * time.Millisecond, DetectMultiplier: 3}

	instance.ensure(t, config, testRouterID, peer)

	assert.NotEmpty(t, instance.peerStatus(t, peer.Address).BFDState, "the BFD change must be applied")

	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		status := instance.peerStatus(t, peer.Address)

		require.Equal(t, nethelpers.BGPSessionStateEstablished, status.State, "BGP session must stay up through an in-place update")
		require.Equal(t, since, status.Since, "BGP session must not be reset by an in-place update")
	}

	// the hold time is only negotiated in OPEN: the live session keeps the old one
	assert.NotEqual(t, uint64(30), negotiatedHoldTime(t, speaker))

	// GoBGP resets the session for a multihop change, and the new OPEN carries the updated hold time
	peer.Config.EBGPMultiHop = 2

	instance.ensure(t, config, testRouterID, peer)

	require.Eventually(t, func() bool {
		return negotiatedHoldTime(t, speaker) == 30
	}, 15*time.Second, 10*time.Millisecond, "the multihop and hold time changes must reach GoBGP")

	assert.Equal(t, 1, instance.starts())
}

// negotiatedHoldTime returns the hold time of the speaker's established session with its only peer, or zero.
func negotiatedHoldTime(t *testing.T, speaker *gobgpsrv.BgpServer) uint64 {
	t.Helper()

	var holdTime uint64

	require.NoError(t, speaker.ListPeer(t.Context(), &gobgpapi.ListPeerRequest{}, func(peer *gobgpapi.Peer) {
		if peer.GetState().GetSessionState() == gobgpapi.PeerState_SESSION_STATE_ESTABLISHED {
			holdTime = peer.GetTimers().GetState().GetNegotiatedHoldTime()
		}
	}))

	return holdTime
}

func TestDerivedRouterIDIsKeptWhilePeerIsEstablished(t *testing.T) {
	t.Parallel()

	derived := network.BGPInstanceConfigSpec{}
	configured := network.BGPInstanceConfigSpec{RouterID: netip.MustParseAddr("10.5.0.9")}
	addrs := func(addrs ...string) []netip.Addr { return xslices.Map(addrs, netip.MustParseAddr) }

	for _, test := range []struct {
		name string

		initial    network.BGPInstanceConfigSpec
		initialID  string
		deriveSame bool
		openOnly   bool
		dropPeer   bool
		removePeer bool
		next       network.BGPInstanceConfigSpec
		candidates []netip.Addr

		expectedID string
	}{
		{
			name:      "lower address added",
			initial:   derived,
			initialID: "10.5.0.3",
			next:      derived, candidates: addrs("10.5.0.2", "10.5.0.3"),
			expectedID: "10.5.0.3",
		},
		{
			name:      "address removed",
			initial:   derived,
			initialID: "10.5.0.3",
			next:      derived, candidates: addrs("10.5.0.2"),
			expectedID: "10.5.0.2",
		},
		{
			name:      "server rebuilt anyway",
			initial:   derived,
			initialID: "10.5.0.3",
			next:      network.BGPInstanceConfigSpec{Multipath: true}, candidates: addrs("10.5.0.2", "10.5.0.3"),
			expectedID: "10.5.0.2",
		},
		{
			name:      "no session established",
			initial:   derived,
			initialID: "10.5.0.3",
			dropPeer:  true,
			next:      derived, candidates: addrs("10.5.0.2", "10.5.0.3"),
			expectedID: "10.5.0.2",
		},
		{
			name:       "configured router ID dropped for the same derived one",
			initial:    network.BGPInstanceConfigSpec{RouterID: netip.MustParseAddr("10.5.0.3")},
			initialID:  "10.5.0.3",
			deriveSame: true,
			next:       derived, candidates: addrs("10.5.0.2", "10.5.0.3"),
			expectedID: "10.5.0.3",
		},
		{
			name:      "session opening",
			initial:   derived,
			initialID: "10.5.0.3",
			openOnly:  true,
			next:      derived, candidates: addrs("10.5.0.2", "10.5.0.3"),
			expectedID: "10.5.0.3",
		},
		{
			name:       "established peer removed",
			initial:    derived,
			initialID:  "10.5.0.3",
			removePeer: true,
			next:       derived, candidates: addrs("10.5.0.2", "10.5.0.3"),
			expectedID: "10.5.0.2",
		},
		{
			name:      "router ID configured",
			initial:   derived,
			initialID: "10.5.0.3",
			next:      configured, candidates: addrs("10.5.0.3"),
			expectedID: "10.5.0.9",
		},
		{
			name:      "configured router ID removed",
			initial:   configured,
			initialID: "10.5.0.9",
			next:      derived, candidates: addrs("10.5.0.3", "10.5.0.9"),
			expectedID: "10.5.0.3",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			instance := newTestInstance(t)
			peer := testPeer("127.0.0.2", 65000)

			ensure := func(config network.BGPInstanceConfigSpec, candidates []netip.Addr, peers ...internalbgp.Peer) {
				t.Helper()

				config.LocalASN = 65001

				instance.ensure(t, config, candidates, peers...)
			}

			ensure(test.initial, addrs(test.initialID), peer)

			if test.deriveSame {
				ensure(derived, addrs(test.initialID), peer)
			}

			if test.openOnly {
				// a peer which sends its OPEN but never a KEEPALIVE holds the session in OpenConfirm
				//
				// The peer closes connections accepted before its FSM leaves Idle, and this one is never redialed.
				instance.waitState(t, peer.Address, nethelpers.BGPSessionStateActive)

				conn, err := (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP(peer.Address)}}).DialContext(
					t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(instance.port))),
				)
				require.NoError(t, err)
				t.Cleanup(func() { conn.Close() }) //nolint:errcheck

				open, err := bgppacket.NewBGPOpenMessage(65000, 90, netip.MustParseAddr("10.0.0.2"), nil)
				require.NoError(t, err)

				openBytes, err := open.Serialize()
				require.NoError(t, err)

				_, err = conn.Write(openBytes)
				require.NoError(t, err)

				instance.waitState(t, peer.Address, nethelpers.BGPSessionStateOpenConfirm)

				ensure(test.next, test.candidates, peer)

				assert.Equal(t, test.expectedID, instance.routerID(t))

				return
			}

			speaker := instance.startSpeaker(t, peer.Address, 65000, 65001)
			instance.waitState(t, peer.Address, nethelpers.BGPSessionStateEstablished)

			if test.dropPeer {
				require.NoError(t, speaker.DeletePeer(t.Context(), &gobgpapi.DeletePeerRequest{Address: "127.0.0.1"}))
				require.Eventually(t, func() bool {
					return instance.peerStatus(t, peer.Address).State != nethelpers.BGPSessionStateEstablished
				}, 5*time.Second, 10*time.Millisecond, "BGP session did not go down")
			}

			nextPeers := []internalbgp.Peer{peer}
			if test.removePeer {
				nextPeers = nil
			}

			// the decision to keep the router ID is logged once, not on every reconcile
			ensure(test.next, test.candidates, nextPeers...)
			ensure(test.next, test.candidates, nextPeers...)

			kept := 0
			if test.expectedID == test.initialID {
				kept = 1
			}

			assert.Equal(t, kept, instance.logs.FilterMessageSnippet("keeping the BGP router ID").Len())
			assert.Equal(t, test.expectedID, instance.routerID(t))
		})
	}
}

// TestEBGPPacketTTL verifies the configured and default eBGP TTL on the wire.
func TestEBGPPacketTTL(t *testing.T) {
	for _, ttl := range []uint32{0, 2, 42, 255} {
		t.Run("TTL"+strconv.FormatUint(uint64(ttl), 10), func(t *testing.T) {
			testEBGPPacketTTL(t, ttl)
		})
	}
}

func testEBGPPacketTTL(t *testing.T, ttl uint32) {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_TCP)
	if err != nil {
		t.Skipf("cannot open raw socket: %v", err)

		return
	}

	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck

	tv := unix.NsecToTimeval(int64(100 * time.Millisecond))
	require.NoError(t, unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv))

	port := int32(testport.Listenable(t))

	instance := internalbgp.NewInstance()
	t.Cleanup(instance.Stop)

	require.NoError(t, instance.EnsureServer(
		t.Context(),
		zap.NewNop(),
		network.BGPInstanceConfigSpec{LocalASN: 65001},
		testRouterID,
		0,
		[]internalbgp.Peer{{
			Address: "127.0.0.2",
			Config: network.BGPNeighborConfigSpec{
				Address:      netip.MustParseAddr("127.0.0.2"),
				PeerASN:      65002,
				EBGPMultiHop: ttl,
			},
		}},
		nil,
		port,
		func() {},
	))

	buf := make([]byte, 65536)

	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				continue
			}

			require.NoError(t, err)
		}

		pkt := gopacket.NewPacket(buf[:n], layers.LayerTypeIPv4, gopacket.Default)

		ip, _ := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
		tcp, _ := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP)

		if ip == nil || tcp == nil || tcp.DstPort != 179 || ip.DstIP.String() != "127.0.0.2" {
			continue
		}

		assert.EqualValues(t, max(uint32(1), ttl), ip.TTL, "eBGP packet must use the configured TTL")

		return
	}

	t.Fatal("did not observe a BGP packet from the listener")
}
