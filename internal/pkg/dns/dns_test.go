// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns_test

import (
	"net"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	dnssrv "github.com/miekg/dns"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/pkg/dns"
)

func TestDNS(t *testing.T) {
	tests := []struct {
		name         string
		nameservers  []string
		expectedCode int
		errCheck     check.Check
	}{
		{
			name:         "success",
			nameservers:  []string{"8.8.8.8"},
			expectedCode: dnssrv.RcodeSuccess,
			errCheck:     check.NoError(),
		},
		{
			name:        "failure",
			nameservers: []string{"242.242.242.242"},
			errCheck:    check.ErrorContains("i/o timeout"),
		},
		{
			name:         "empty destinations",
			nameservers:  nil,
			expectedCode: dnssrv.RcodeServerFailure,
			errCheck:     check.NoError(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stop := newServer(t, test.nameservers...)
			defer stop()

			time.Sleep(10 * time.Millisecond)

			r, err := dnssrv.Exchange(createQuery(), "127.0.0.53:10700")
			test.errCheck(t, err)

			if r != nil {
				require.Equal(t, test.expectedCode, r.Rcode, r)
			}

			t.Logf("r: %s", r)

			stop()
		})
	}
}

func TestDNSEmptyDestinations(t *testing.T) {
	stop := newServer(t)
	defer stop()

	time.Sleep(10 * time.Millisecond)

	r, err := dnssrv.Exchange(createQuery(), "127.0.0.53:10700")
	require.NoError(t, err)
	require.Equal(t, dnssrv.RcodeServerFailure, r.Rcode, r)

	r, err = dnssrv.Exchange(createQuery(), "127.0.0.53:10700")
	require.NoError(t, err)
	require.Equal(t, dnssrv.RcodeServerFailure, r.Rcode, r)

	stop()
}

func newServer(t *testing.T, nameservers ...string) func() {
	l := zaptest.NewLogger(t)

	handler := dns.NewHandler(l)
	t.Cleanup(handler.Stop)

	pxs := xslices.Map(nameservers, func(ns string) *proxy.Proxy {
		p := proxy.NewProxy(ns, net.JoinHostPort(ns, "53"), "dns")
		p.Start(500 * time.Millisecond)

		t.Cleanup(p.Stop)

		return p
	})

	handler.SetProxy(pxs)

	pc, err := dns.NewUDPPacketConn("udp", "127.0.0.53:10700")
	require.NoError(t, err)

	srv := dns.NewServer(dns.ServerOptions{
		PacketConn: pc,
		Handler:    dns.NewCache(handler, l),
		Logger:     l,
	})

	stop, _ := srv.Start(func(err error) {
		if err != nil {
			t.Errorf("error running dns server: %v", err)
		}

		t.Logf("dns server stopped")
	})

	return stop
}

func createQuery() *dnssrv.Msg {
	return &dnssrv.Msg{
		MsgHdr: dnssrv.MsgHdr{
			Id:               dnssrv.Id(),
			RecursionDesired: true,
		},
		Question: []dnssrv.Question{
			{
				Name:   dnssrv.Fqdn("google.com"),
				Qtype:  dnssrv.TypeA,
				Qclass: dnssrv.ClassINET,
			},
		},
	}
}
