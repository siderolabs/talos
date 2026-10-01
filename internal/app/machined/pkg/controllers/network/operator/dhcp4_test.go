// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator_test

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

var (
	dhcpServerID = net.IPv4(192, 168, 50, 1).To4()
	dhcpFirstIP  = net.IPv4(192, 168, 50, 101).To4()
	dhcpSecondIP = net.IPv4(192, 168, 50, 150).To4()
	dhcpHWAddr   = net.HardwareAddr{0x02, 0, 0, 0, 0, 0x01}
)

// fakeDHCPServer offers the reserved address, ACKs requests for it and NAKs everything else.
type fakeDHCPServer struct {
	mu          sync.Mutex
	reservation net.IP
	silent      bool
	naks        int
	received    []*dhcpv4.DHCPv4
}

func (srv *fakeDHCPServer) setReservation(ip net.IP) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	srv.reservation = ip
}

func (srv *fakeDHCPServer) setSilent(silent bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	srv.silent = silent
}

func (srv *fakeDHCPServer) stats() (received []*dhcpv4.DHCPv4, naks int) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	return append([]*dhcpv4.DHCPv4(nil), srv.received...), srv.naks
}

func (srv *fakeDHCPServer) handle(req *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	srv.received = append(srv.received, req)

	if srv.silent {
		return nil
	}

	var mods []dhcpv4.Modifier

	switch req.MessageType() { //nolint:exhaustive
	case dhcpv4.MessageTypeDiscover:
		mods = append(mods, dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer), dhcpv4.WithYourIP(srv.reservation))
	case dhcpv4.MessageTypeRequest:
		requested := req.RequestedIPAddress()
		if requested == nil { // RENEWING
			requested = req.ClientIPAddr
		}

		if requested.Equal(srv.reservation) {
			mods = append(mods, dhcpv4.WithMessageType(dhcpv4.MessageTypeAck), dhcpv4.WithYourIP(srv.reservation))
		} else {
			srv.naks++

			mods = append(mods, dhcpv4.WithMessageType(dhcpv4.MessageTypeNak))
		}
	default:
		return nil
	}

	resp, err := dhcpv4.NewReplyFromRequest(req, append(
		mods,
		dhcpv4.WithServerIP(dhcpServerID),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(dhcpServerID)),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(10*time.Minute)),
		dhcpv4.WithRouter(dhcpServerID),
	)...)
	if err != nil {
		panic(err)
	}

	return resp
}

// newClient implements operator.DHCP4ClientFactory.
//
// The options (e.g. unicast to the server on renewal) are ignored, as all packets go to the fake server.
func (srv *fakeDHCPServer) newClient(string, ...nclient4.ClientOpt) (*nclient4.Client, error) {
	return nclient4.NewWithConn(&fakeDHCPConn{
		srv:    srv,
		rx:     make(chan []byte, 16),
		closed: make(chan struct{}),
	}, dhcpHWAddr)
}

// fakeDHCPConn delivers the packets to the fake server in memory, so that it works in a synctest bubble.
type fakeDHCPConn struct {
	srv *fakeDHCPServer

	rx        chan []byte
	closeOnce sync.Once
	closed    chan struct{}
}

func (c *fakeDHCPConn) ReadFrom(b []byte) (int, net.Addr, error) {
	select {
	case p := <-c.rx:
		return copy(b, p), &net.UDPAddr{IP: dhcpServerID, Port: nclient4.ServerPort}, nil
	case <-c.closed:
		return 0, nil, net.ErrClosed
	}
}

func (c *fakeDHCPConn) WriteTo(b []byte, _ net.Addr) (int, error) {
	req, err := dhcpv4.FromBytes(b)
	if err != nil {
		return 0, err
	}

	if resp := c.srv.handle(req); resp != nil {
		c.rx <- resp.ToBytes()
	}

	return len(b), nil
}

func (c *fakeDHCPConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })

	return nil
}

func (c *fakeDHCPConn) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4zero, Port: nclient4.ClientPort}
}

func (c *fakeDHCPConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeDHCPConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeDHCPConn) SetWriteDeadline(time.Time) error { return nil }

type testPlatform struct {
	runtime.Platform
}

func (testPlatform) Name() string { return "metal" }

// runDHCP4 runs the operator against the fake server, and reports the network as ready for the given addresses.
func runDHCP4(t *testing.T, srv *fakeDHCPServer, readyAddresses ...string) (*operator.DHCP4, <-chan struct{}) {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	for _, addr := range readyAddresses {
		require.NoError(t, st.Create(t.Context(), network.NewAddressStatus(network.NamespaceName, network.AddressID("eth0", netip.MustParsePrefix(addr)))))
	}

	status := network.NewStatus(network.NamespaceName, network.StatusID)
	status.TypedSpec().AddressReady = true
	status.TypedSpec().ConnectivityReady = true
	require.NoError(t, st.Create(t.Context(), status))

	d := operator.NewDHCP4(zaptest.NewLogger(t), "eth0", network.DHCP4OperatorSpec{}, testPlatform{}, st, srv.newClient)

	ctx, cancel := context.WithCancel(t.Context())
	notifyCh := make(chan struct{})

	var wg sync.WaitGroup

	wg.Go(func() { d.Run(ctx, notifyCh) })

	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	return d, notifyCh
}

func assertDHCP4Address(t *testing.T, d *operator.DHCP4, expected string) {
	t.Helper()

	addresses := d.AddressSpecs()

	require.Len(t, addresses, 1)
	assert.Equal(t, netip.MustParsePrefix(expected), addresses[0].Address)
	assert.NotEmpty(t, d.RouteSpecs())
}

// TestDHCP4ChangedReservation verifies that the lease rejected on renewal is dropped, and a new one is acquired.
func TestDHCP4ChangedReservation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := &fakeDHCPServer{reservation: dhcpFirstIP}
		d, notifyCh := runDHCP4(t, srv, "192.168.50.101/24", "192.168.50.150/24")

		<-notifyCh
		assertDHCP4Address(t, d, "192.168.50.101/24")

		// the reservation changes, and the server forgets the previous lease
		srv.setReservation(dhcpSecondIP)

		// the renewal is rejected
		<-notifyCh
		assert.Empty(t, d.AddressSpecs(), "rejected address should be dropped")
		assert.Empty(t, d.RouteSpecs(), "routes sourced from the rejected address should be dropped")

		// the configuration process restarts
		<-notifyCh
		assertDHCP4Address(t, d, "192.168.50.150/24")

		received, naks := srv.stats()
		assert.Equal(t, 1, naks)

		require.Len(t, received, 5)

		assert.Equal(t, dhcpv4.MessageTypeDiscover, received[0].MessageType())
		assert.Equal(t, dhcpv4.MessageTypeRequest, received[1].MessageType())
		assert.Equal(t, dhcpFirstIP, received[1].RequestedIPAddress().To4())

		// renewal
		assert.Equal(t, dhcpv4.MessageTypeRequest, received[2].MessageType())
		assert.Equal(t, dhcpFirstIP, received[2].ClientIPAddr.To4())

		assert.Equal(t, dhcpv4.MessageTypeDiscover, received[3].MessageType())
		assert.False(t, received[3].Options.Has(dhcpv4.OptionRequestedIPAddress), "rejected address should not be requested again")
		assert.Equal(t, dhcpv4.MessageTypeRequest, received[4].MessageType())
		assert.Equal(t, dhcpSecondIP, received[4].RequestedIPAddress().To4())
	})
}

// TestDHCP4ServerDown verifies that the address is kept while the server is down, and the previous address
// is only a hint in the DISCOVER, while the REQUEST follows the OFFER.
func TestDHCP4ServerDown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := &fakeDHCPServer{reservation: dhcpFirstIP}
		d, notifyCh := runDHCP4(t, srv, "192.168.50.101/24", "192.168.50.150/24")

		<-notifyCh
		assertDHCP4Address(t, d, "192.168.50.101/24")

		srv.setSilent(true)

		// renewal and a number of the discovery attempts fail
		time.Sleep(time.Hour)
		synctest.Wait()

		assertDHCP4Address(t, d, "192.168.50.101/24")

		received, _ := srv.stats()

		last := received[len(received)-1]
		assert.Equal(t, dhcpv4.MessageTypeDiscover, last.MessageType())
		assert.Equal(t, dhcpFirstIP, last.RequestedIPAddress().To4())

		// the server comes back, but the previous address is not available anymore
		srv.setReservation(dhcpSecondIP)
		srv.setSilent(false)

		<-notifyCh
		assertDHCP4Address(t, d, "192.168.50.150/24")

		received, naks := srv.stats()
		assert.Zero(t, naks)

		discover, request := received[len(received)-2], received[len(received)-1]

		assert.Equal(t, dhcpv4.MessageTypeDiscover, discover.MessageType())
		assert.Equal(t, dhcpFirstIP, discover.RequestedIPAddress().To4())

		assert.Equal(t, dhcpv4.MessageTypeRequest, request.MessageType())
		assert.Equal(t, dhcpSecondIP, request.RequestedIPAddress().To4())
		assert.Equal(t, dhcpServerID, request.ServerIdentifier().To4())
	})
}
