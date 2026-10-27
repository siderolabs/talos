// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"context"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/jsimonetti/rtnetlink/v2"
	gobgpapi "github.com/osrg/gobgp/v4/api"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// TestNumberedBFDSource checks packets, not just configuration: an unspecified
// BFD source must follow the established TCP socket, including accepted sockets
// whose local address differs from the kernel's preferred outgoing source.
func TestNumberedBFDSource(t *testing.T) {
	for _, family := range []string{"IPv4", "IPv6"} {
		for _, direction := range []string{"accepted", "outgoing"} {
			t.Run(family+"/"+direction, func(t *testing.T) {
				testNumberedBFDSource(t, family == "IPv6", direction == "outgoing")
			})
		}
	}
}

func assertNumberedFixtureRemoved(t *testing.T, conn *rtnetlink.Conn, ips ...string) {
	t.Helper()

	addresses, err := conn.Address.List()
	require.NoError(t, err)
	routes, err := conn.Route.List()
	require.NoError(t, err)

	for _, ip := range ips {
		for _, address := range addresses {
			require.False(t, address.Attributes.Address.Equal(net.ParseIP(ip)), "fixture address leaked: %s", ip)
		}

		for _, route := range routes {
			require.False(t, route.Attributes.Dst.Equal(net.ParseIP(ip)), "fixture route leaked: %s: %+v", ip, route)
		}
	}
}

func assertIPv6FixtureReady(t *testing.T, conn *rtnetlink.Conn, index uint32, address net.IP) {
	t.Helper()

	addresses, err := conn.Address.List()
	require.NoError(t, err)

	for _, actual := range addresses {
		if actual.Index == index && actual.Attributes.Address.Equal(address) {
			require.NotZero(t, actual.Attributes.Flags&unix.IFA_F_NODAD, "IPv6 fixture must disable DAD")
			require.Zero(t, actual.Attributes.Flags&unix.IFA_F_TENTATIVE, "IPv6 fixture must be ready immediately")

			return
		}
	}

	t.Fatal("IPv6 fixture address must exist")
}

func testNumberedBFDSource(t *testing.T, ipv6, outgoing bool) {
	t.Helper()

	primary, secondary, remoteIP, tcp, udp := "192.0.2.20", "192.0.2.10", "192.0.2.99", "tcp4", "udp4"
	family, prefixLength := uint8(unix.AF_INET), uint8(32)
	priority := uint32(0)

	if ipv6 {
		primary, secondary, remoteIP, tcp, udp = "2001:db8::20", "2001:db8::10", "2001:db8::99", "tcp6", "udp6"
		family, prefixLength = unix.AF_INET6, 128
		// IPv6 assigns this default metric when none is supplied. Include it
		// explicitly so deletion selects our route, not the address's route.
		priority = 1024
	}

	// All addresses and routes exist only in the disposable child namespace.
	conn, err := rtnetlink.Dial(nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	lo, err := net.InterfaceByName("lo")
	require.NoError(t, err)

	index := uint32(lo.Index)
	require.NoError(t, conn.Link.Set(&rtnetlink.LinkMessage{
		Index: index, Flags: unix.IFF_UP, Change: unix.IFF_UP,
	}))

	// Registered before fixture cleanup, this runs after it and catches leaks
	// even in the last case, when no subsequent test would encounter them.
	t.Cleanup(func() {
		assertNumberedFixtureRemoved(t, conn, primary, secondary, remoteIP)
	})

	for _, ip := range []string{primary, secondary, remoteIP} {
		address := netip.MustParseAddr(ip).AsSlice()
		message := &rtnetlink.AddressMessage{
			Family: family, PrefixLength: prefixLength, Index: index,
			Attributes: &rtnetlink.AddressAttributes{Address: address, Local: address, Flags: unix.IFA_F_NODAD},
		}
		require.NoError(t, conn.Address.New(message))
		// Cleanup runs after the sockets, server, and explicit route cleanup,
		// and before the netlink connection closes.
		t.Cleanup(func() { require.NoError(t, conn.Address.Delete(message)) })

		if ipv6 {
			assertIPv6FixtureReady(t, conn, index, address)
		}
	}

	route := &rtnetlink.RouteMessage{
		Family: family, DstLength: prefixLength,
		Table: unix.RT_TABLE_LOCAL, Type: unix.RTN_LOCAL, Scope: unix.RT_SCOPE_HOST,
		Attributes: rtnetlink.RouteAttributes{
			OutIface: index, Dst: netip.MustParseAddr(remoteIP).AsSlice(), Src: netip.MustParseAddr(primary).AsSlice(), Priority: priority,
		},
	}
	require.NoError(t, conn.Route.Replace(route))
	t.Cleanup(func() { require.NoError(t, conn.Route.Delete(route)) })

	packets, err := net.ListenUDP(udp, &net.UDPAddr{IP: net.ParseIP(remoteIP)})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, packets.Close()) })

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	server := startBGPImportTestServer(t, ctx, 65001, "192.0.2.1", 179, false)
	peer := internalbgp.BuildPeer(internalbgp.Peer{Address: remoteIP, Config: network.BGPNeighborConfigSpec{
		Address: netip.MustParseAddr(remoteIP), PeerASN: 65002, Passive: !outgoing, BFD: &network.BGPBFDConfigSpec{},
	}}, false)
	require.Empty(t, peer.GetTransport().GetLocalAddress())

	peer.Bfd.Port = uint32(packets.LocalAddr().(*net.UDPAddr).Port)
	if peer.Transport == nil {
		peer.Transport = &gobgpapi.Transport{}
	}

	var connection net.Conn

	if outgoing {
		listener, listenErr := net.ListenTCP(tcp, &net.TCPAddr{IP: net.ParseIP(remoteIP)})
		require.NoError(t, listenErr)
		t.Cleanup(func() { require.NoError(t, listener.Close()) })

		peer.Transport.RemotePort = uint32(listener.Addr().(*net.TCPAddr).Port)
		peer.Timers = &gobgpapi.Timers{Config: &gobgpapi.TimersConfig{ConnectRetry: 1}}
		require.NoError(t, server.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: peer}))
		require.NoError(t, listener.SetDeadline(time.Now().Add(10*time.Second)))
		connection, err = listener.AcceptTCP()
	} else {
		require.NoError(t, server.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: peer}))
		require.Eventually(t, func() bool {
			ready := false
			listErr := server.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(p *gobgpapi.Peer) {
				ready = p.GetState().GetSessionState() == gobgpapi.PeerState_SESSION_STATE_ACTIVE
			})

			return listErr == nil && ready
		}, 5*time.Second, time.Millisecond)

		dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP(remoteIP)}}
		connection, err = dialer.DialContext(ctx, tcp, net.JoinHostPort(secondary, "179"))
	}

	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	require.NoError(t, connection.SetDeadline(time.Now().Add(10*time.Second)))

	open, err := bgppacket.NewBGPOpenMessage(65002, 90, netip.MustParseAddr("192.0.2.99"), nil)
	require.NoError(t, err)

	for _, message := range []*bgppacket.BGPMessage{open, bgppacket.NewBGPKeepAliveMessage()} {
		wire, serializeErr := message.Serialize()
		require.NoError(t, serializeErr)

		_, err = connection.Write(wire)
		require.NoError(t, err)
	}

	go func() { _, _ = io.Copy(io.Discard, connection) }() //nolint:errcheck // Drain BGP messages until the test closes the connection.

	require.Eventually(t, func() bool {
		established := false
		listErr := server.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(p *gobgpapi.Peer) {
			established = p.GetState().GetSessionState() == gobgpapi.PeerState_SESSION_STATE_ESTABLISHED
		})

		return listErr == nil && established
	}, 5*time.Second, time.Millisecond)

	tcpSource := connection.RemoteAddr().(*net.TCPAddr).IP.String()
	want := secondary

	if outgoing {
		// IPv6 local routes can prefer the destination's own address. Check
		// the kernel's choice rather than assuming IPv4 source-selection rules.
		probe, probeErr := net.DialUDP(udp, nil, packets.LocalAddr().(*net.UDPAddr))
		require.NoError(t, probeErr)

		want = probe.LocalAddr().(*net.UDPAddr).IP.String()
		require.NoError(t, probe.Close())
	}

	t.Logf("BGP Established: TCP source=%s expected=%s", tcpSource, want)
	require.Equal(t, want, tcpSource)
	require.NoError(t, packets.SetReadDeadline(time.Now().Add(5*time.Second)))

	buffer := make([]byte, 512)
	length, source, err := packets.ReadFromUDP(buffer)
	require.NoError(t, err)
	t.Logf("BFD packet: source=%s expected=%s bytes=%d", source, tcpSource, length)
	require.GreaterOrEqual(t, length, 24)
	require.Equal(t, tcpSource, source.IP.String())
	require.GreaterOrEqual(t, source.Port, 49152)
}
