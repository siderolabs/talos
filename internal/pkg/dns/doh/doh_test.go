// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package doh_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/dns/doh"
)

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
