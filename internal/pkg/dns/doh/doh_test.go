// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package doh_test

import (
	"context"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/dns/doh"
)

// dohServer is a DoH server answering with an A record, or with the HTTP status if set.
type dohServer struct {
	status atomic.Int32
	proto  atomic.Value
	ids    []uint16
}

func (s *dohServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.proto.Store(r.Proto)

	if r.Method != http.MethodPost || r.URL.Path != "/dns-query" || r.Header.Get("Content-Type") != "application/dns-message" {
		http.Error(w, "bad request", http.StatusBadRequest)

		return
	}

	if status := s.status.Load(); status != 0 {
		http.Error(w, "failed", int(status))

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	req := new(dns.Msg)
	if err = req.Unpack(body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	s.ids = append(s.ids, req.Id)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.IPv4(10, 0, 0, 1),
	})

	wire, err := resp.Pack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/dns-message")
	w.Write(wire) //nolint:errcheck
}

func TestExchange(t *testing.T) {
	handler := &dohServer{}

	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.StartTLS()

	t.Cleanup(server.Close)

	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())

	// the test certificate is valid for "example.com"
	p := doh.NewProxy(server.Listener.Addr().String(), "example.com")
	t.Cleanup(p.Close)

	p.SetRootCAs(pool)

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	req.Id = 1234

	resp, err := p.Exchange(t.Context(), req, "udp")
	require.NoError(t, err)

	// the original ID is restored
	assert.EqualValues(t, 1234, resp.Id)
	require.Len(t, resp.Answer, 1)
	assert.Equal(t, "example.com.\t60\tIN\tA\t10.0.0.1", resp.Answer[0].String())
	assert.Zero(t, p.Fails())

	// the ID is zero on the wire, and the request is sent over HTTP/2
	assert.Equal(t, []uint16{0}, handler.ids)
	assert.Equal(t, "HTTP/2.0", handler.proto.Load())

	// HTTP errors are reported and counted
	handler.status.Store(http.StatusServiceUnavailable)

	_, err = p.Exchange(t.Context(), req, "udp")
	require.ErrorContains(t, err, "HTTP 503")
	assert.EqualValues(t, 1, p.Fails())

	// the proxy recovers
	handler.status.Store(0)

	_, err = p.Exchange(t.Context(), req, "udp")
	require.NoError(t, err)
	assert.Zero(t, p.Fails())

	// a wrong server name fails the TLS verification (the test certificate covers *.example.com)
	bad := doh.NewProxy(server.Listener.Addr().String(), "dns.example.org")
	t.Cleanup(bad.Close)

	bad.SetRootCAs(pool)

	_, err = bad.Exchange(t.Context(), req, "udp")
	require.Error(t, err)
}

// TestDialContextRouting verifies that the transport's DialContext redirects
// direct dials to the upstream's pre-resolved IP, while passing through any
// other address unchanged (i.e. the http.Transport.Proxy hand-off when
// HTTPS_PROXY is set).
func TestDialContextRouting(t *testing.T) {
	const serverName = "doh.example.test"

	upstreamLn, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { upstreamLn.Close() }) //nolint:errcheck

	proxyLn, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { proxyLn.Close() }) //nolint:errcheck

	upstreamAddr := upstreamLn.Addr().String()
	proxyAddr := proxyLn.Addr().String()

	accepted := make(chan string, 2)

	accept := func(ln net.Listener, label string) {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			accepted <- label

			conn.Close() //nolint:errcheck
		}
	}

	go accept(upstreamLn, "upstream")
	go accept(proxyLn, "proxy")

	p := doh.NewProxy(upstreamAddr, serverName)
	t.Cleanup(p.Close)

	for _, tc := range []struct {
		name    string
		address string
		want    string
	}{
		{
			name:    "direct dial to serverName redirects to upstream IP",
			address: net.JoinHostPort(serverName, "443"),
			want:    "upstream",
		},
		{
			name:    "dial to proxy address passes through",
			address: proxyAddr,
			want:    "proxy",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			conn, err := p.DialContext(ctx, "tcp", tc.address)
			require.NoError(t, err)

			t.Cleanup(func() { conn.Close() }) //nolint:errcheck

			select {
			case got := <-accepted:
				assert.Equal(t, tc.want, got)
			case <-ctx.Done():
				t.Fatal("no listener accepted the connection")
			}
		})
	}
}
