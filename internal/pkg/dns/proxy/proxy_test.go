// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxy_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/siderolabs/talos/internal/pkg/dns/proxy"
)

// testServer is a DNS server answering with A records, counting the queries and connections.
type testServer struct {
	mu       sync.Mutex
	ids      []uint16
	udpPorts []int

	connections atomic.Int32
	answers     int
}

func (s *testServer) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	s.mu.Lock()
	s.ids = append(s.ids, req.Id)

	if addr, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		s.udpPorts = append(s.udpPorts, addr.Port)
	}

	s.mu.Unlock()

	resp := new(dns.Msg)
	resp.SetReply(req)

	for i := range max(s.answers, 1) {
		resp.Answer = append(resp.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.IPv4(10, 0, byte(i/256), byte(i%256)),
		})
	}

	w.WriteMsg(resp) //nolint:errcheck
}

func (s *testServer) seenIDs() []uint16 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]uint16(nil), s.ids...)
}

func (s *testServer) seenUDPPorts() []int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]int(nil), s.udpPorts...)
}

// countingListener counts the accepted connections.
type countingListener struct {
	net.Listener

	count *atomic.Int32
}

func (l *countingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.count.Add(1)
	}

	return conn, err
}

// startServer starts UDP and TCP servers on the same port (or a TLS server if tlsConfig is set).
func startServer(t *testing.T, handler *testServer, tlsConfig *tls.Config, idleTimeout time.Duration) string {
	t.Helper()

	tcpListener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	var listener net.Listener = &countingListener{Listener: tcpListener, count: &handler.connections}

	if tlsConfig != nil {
		listener = tls.NewListener(listener, tlsConfig)
	}

	tcpServer := &dns.Server{
		Listener: listener,
		Handler:  handler,
		IdleTimeout: func() time.Duration {
			return idleTimeout
		},
	}

	addr := tcpListener.Addr().String()

	servers := []*dns.Server{tcpServer}

	if tlsConfig == nil {
		packetConn, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp4", addr)
		require.NoError(t, err)

		servers = append(servers, &dns.Server{PacketConn: packetConn, Handler: handler})
	}

	for _, srv := range servers {
		started := make(chan struct{})
		srv.NotifyStartedFunc = func() { close(started) }

		go srv.ActivateAndServe() //nolint:errcheck

		<-started

		t.Cleanup(func() { srv.Shutdown() }) //nolint:errcheck
	}

	return addr
}

func query(id uint16) *dns.Msg {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	req.Id = id

	return req
}

func TestExchange(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	handler := &testServer{}
	addr := startServer(t, handler, nil, time.Minute)

	p := proxy.New(addr, nil)
	p.Start(100 * time.Millisecond)

	defer p.Close()

	assert.Equal(t, addr, p.Addr())

	for _, proto := range []string{"udp", "tcp", "udp", "tcp", "tcp"} {
		resp, err := p.Exchange(t.Context(), query(1234), proto)
		require.NoError(t, err)

		// the original ID is restored
		assert.EqualValues(t, 1234, resp.Id)
		require.Len(t, resp.Answer, 1)
		assert.Equal(t, "example.com.\t60\tIN\tA\t10.0.0.0", resp.Answer[0].String())
	}

	// the query ID is randomized for every upstream query
	ids := handler.seenIDs()
	require.Len(t, ids, 5)

	distinct := map[uint16]struct{}{}
	for _, id := range ids {
		distinct[id] = struct{}{}
	}

	assert.Greater(t, len(distinct), 1, "query IDs should be randomized: %v", ids)

	// TCP connection is reused
	assert.EqualValues(t, 1, handler.connections.Load())

	// UDP sockets are not reused, so the source port is randomized for every query
	ports := handler.seenUDPPorts()
	require.Len(t, ports, 2)
	assert.NotEqual(t, ports[0], ports[1], "UDP source ports should be different: %v", ports)
}

func TestExchangeClosedByServer(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	handler := &testServer{}
	// the server closes idle connections quickly
	addr := startServer(t, handler, nil, 50*time.Millisecond)

	p := proxy.New(addr, nil)
	defer p.Close()

	for range 3 {
		resp, err := p.Exchange(t.Context(), query(1), "tcp")
		require.NoError(t, err)
		require.Len(t, resp.Answer, 1)

		// wait for the server to close the idle connection
		time.Sleep(200 * time.Millisecond)
	}

	// each query required a new connection, as the previous one was closed by the server
	assert.EqualValues(t, 3, handler.connections.Load())
}

func TestExchangeTruncated(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	// the response doesn't fit into 512 bytes
	handler := &testServer{answers: 100}
	addr := startServer(t, handler, nil, time.Minute)

	p := proxy.New(addr, nil)
	defer p.Close()

	// the UDP response is truncated, so the query is retried over TCP
	resp, err := p.Exchange(t.Context(), query(42), "udp")
	require.NoError(t, err)

	assert.False(t, resp.Truncated)
	assert.Len(t, resp.Answer, 100)
	assert.EqualValues(t, 42, resp.Id)

	assert.Len(t, handler.seenUDPPorts(), 1)
	assert.Len(t, handler.seenIDs(), 2)
	assert.EqualValues(t, 1, handler.connections.Load())

	// with a larger EDNS0 buffer, the response fits into UDP
	req := query(43)
	req.SetEdns0(4096, false)

	resp, err = p.Exchange(t.Context(), req, "udp")
	require.NoError(t, err)

	assert.False(t, resp.Truncated)
	assert.Len(t, resp.Answer, 100)

	assert.Len(t, handler.seenUDPPorts(), 2)
	assert.EqualValues(t, 1, handler.connections.Load())

	// over TCP, the response is complete
	resp, err = p.Exchange(t.Context(), query(44), "tcp")
	require.NoError(t, err)

	assert.False(t, resp.Truncated)
	assert.Len(t, resp.Answer, 100)
}

func TestExchangeTLS(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	serverConfig, clientConfig := tlsConfigs(t)

	handler := &testServer{}
	addr := startServer(t, handler, serverConfig, time.Minute)

	p := proxy.New(addr, clientConfig)
	defer p.Close()

	for range 3 {
		// DNS-over-TLS ignores the requested protocol
		resp, err := p.Exchange(t.Context(), query(7), "udp")
		require.NoError(t, err)

		assert.EqualValues(t, 7, resp.Id)
		require.Len(t, resp.Answer, 1)
	}

	// TLS connection is reused
	assert.EqualValues(t, 1, handler.connections.Load())

	// health check is done over TLS
	p.Healthcheck()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.GreaterOrEqual(collect, handler.connections.Load(), int32(2))
	}, 5*time.Second, 10*time.Millisecond)

	assert.Zero(t, p.Fails())

	// wrong server name fails the TLS verification
	badConfig := clientConfig.Clone()
	badConfig.ServerName = "other.example.com"

	bad := proxy.New(addr, badConfig)
	defer bad.Close()

	_, err := bad.Exchange(t.Context(), query(8), "tcp")
	require.Error(t, err)
}

func TestHealthcheck(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	handler := &testServer{}
	addr := startServer(t, handler, nil, time.Minute)

	healthy := proxy.New(addr, nil)
	healthy.Start(10 * time.Millisecond)

	defer healthy.Close()

	healthy.Healthcheck()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.NotEmpty(collect, handler.seenIDs())
	}, 5*time.Second, 10*time.Millisecond)

	assert.Zero(t, healthy.Fails())

	// nothing listens on the port: the health check fails
	listener, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp4", "127.0.0.1:0")
	require.NoError(t, err)

	deadAddr := listener.LocalAddr().String()
	require.NoError(t, listener.Close())

	dead := proxy.New(deadAddr, nil)
	dead.Start(10 * time.Millisecond)

	dead.Healthcheck()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.GreaterOrEqual(collect, dead.Fails(), uint32(2))
	}, 5*time.Second, 10*time.Millisecond)

	// the health check is retried until it succeeds, stopped by Close
	dead.Close()
}

func TestClose(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	handler := &testServer{}
	addr := startServer(t, handler, nil, time.Minute)

	p := proxy.New(addr, nil)
	p.Start(time.Second)

	_, err := p.Exchange(t.Context(), query(1), "tcp")
	require.NoError(t, err)

	p.Close()
	p.Close()

	_, err = p.Exchange(t.Context(), query(1), "tcp")
	require.ErrorIs(t, err, proxy.ErrClosed)
}

// tlsConfigs generates a self-signed certificate for "dns.example.com".
func tlsConfigs(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "dns.example.com"},
		DNSNames:              []string{"dns.example.com"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AddCert(cert)

	serverConfig := &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS13,
	}

	clientConfig := &tls.Config{
		ServerName: "dns.example.com",
		RootCAs:    pool,
		MinVersion: tls.VersionTLS13,
	}

	return serverConfig, clientConfig
}
