// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns_test

import (
	"context"
	"errors"
	"iter"
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	dnssrv "github.com/miekg/dns"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/require"
	"github.com/thejerf/suture/v4"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/pkg/dns"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

func TestDNS(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	tests := []struct {
		name         string
		hostname     string
		nameservers  []string
		expectedCode int
		errCheck     check.Check
	}{
		{
			name:         "success",
			hostname:     "google.com",
			nameservers:  []string{"8.8.8.8"},
			expectedCode: dnssrv.RcodeSuccess,
			errCheck:     check.NoError(),
		},
		{
			name:        "failure",
			hostname:    "google-fail.com",
			nameservers: []string{"242.242.242.242"},
			errCheck:    check.ErrorContains("i/o timeout"),
		},
		{
			name:         "empty destinations",
			hostname:     "google.com",
			nameservers:  nil,
			expectedCode: dnssrv.RcodeServerFailure,
			errCheck:     check.NoError(),
		},
		{
			name:         "empty destinations but node exists",
			hostname:     "talos-default-worker-1",
			nameservers:  nil,
			expectedCode: dnssrv.RcodeSuccess,
			errCheck:     check.NoError(),
		},
		{
			name:         "empty destinations but node doesn't exists",
			hostname:     "talos-default-worker-2",
			nameservers:  []string{"8.8.8.8"},
			expectedCode: dnssrv.RcodeNameError,
			errCheck:     check.NoError(),
		},
		{
			name:         "empty destinations but static host exists",
			hostname:     "static-host-1",
			nameservers:  nil,
			expectedCode: dnssrv.RcodeSuccess,
			errCheck:     check.NoError(),
		},
		{
			// The first one will return SERVFAIL and the second will return REFUSED. We should try both.
			name:         `should return "refused"`,
			hostname:     "dnssec-failed.org",
			nameservers:  []string{"1.1.1.1", "ns1.google.com."},
			expectedCode: dnssrv.RcodeRefused,
			errCheck:     check.NoError(),
		},
	}

	for _, dnsAddr := range []string{"127.0.0.1:10700", "[::1]:10700"} {
		for _, test := range tests {
			t.Run(dnsAddr+"/"+test.name, func(t *testing.T) {
				stop := newManager(t, test.nameservers...)
				t.Cleanup(stop)

				time.Sleep(10 * time.Millisecond)

				c := dnssrv.Client{
					Timeout: time.Second,
				}

				r, _, err := c.Exchange(createQuery(test.hostname), dnsAddr)
				test.errCheck(t, err)

				if r != nil {
					require.Equal(t, test.expectedCode, r.Rcode, r)
				}

				t.Logf("r: %s", r)
			})
		}
	}
}

func TestDNSEmptyDestinations(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	stop := newManager(t)
	defer stop()

	time.Sleep(10 * time.Millisecond)

	r, err := dnssrv.Exchange(createQuery("google.com"), "127.0.0.1:10700")
	require.NoError(t, err)
	require.Equal(t, dnssrv.RcodeServerFailure, r.Rcode, r)

	r, err = dnssrv.Exchange(createQuery("google.com"), "127.0.0.1:10700")
	require.NoError(t, err)
	require.Equal(t, dnssrv.RcodeServerFailure, r.Rcode, r)

	r, err = dnssrv.Exchange(createQuery("google.com"), "[::1]:10700")
	require.NoError(t, err)
	require.Equal(t, dnssrv.RcodeServerFailure, r.Rcode, r)

	r, err = dnssrv.Exchange(createQuery("google.com"), "[::1]:10700")
	require.NoError(t, err)
	require.Equal(t, dnssrv.RcodeServerFailure, r.Rcode, r)

	stop()
}

func Test_ServeBackground(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	m := dns.NewManager(&testMemberReader{}, &testStaticHostReader{}, func(e suture.Event) { t.Log("dns-runners event:", e) }, zaptest.NewLogger(t))

	m.ServeBackground(t.Context())

	// should not panic since ServeBackground is called with the same context
	m.ServeBackground(t.Context())

	// should panic since ServeBackground is called with a different context
	require.Panics(t, func() { m.ServeBackground(context.TODO()) }) //nolint:usetesting

	for _, err := range m.RunAll(slices.Values([]dns.AddressPair{
		{Network: "udp", Addr: netip.MustParseAddrPort("127.0.0.1:10700")},
		{Network: "udp", Addr: netip.MustParseAddrPort("127.0.0.1:10701")},
		{Network: "udp", Addr: netip.MustParseAddrPort("[::1]:10700")},
		{Network: "udp", Addr: netip.MustParseAddrPort("[::1]:10701")},
	}), false) {
		require.NoError(t, err)
	}

	require.NoError(t, m.ClearAll(false))
}

// TestRunnerRestart ensures a [dns.Runner] can be started, stopped, and
// started again. Previously the runner reused a single dns.Server across
// restarts, so the second start failed permanently with
// "dns: server already started", leaving nothing bound to the listen address.
func TestRunnerRestart(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	newOpts := func() (dns.RunnerOptions, error) {
		pc, err := dns.NewUDPPacketConn("udp", "127.0.0.1:0", nil)
		if err != nil {
			return dns.RunnerOptions{}, err
		}

		return dns.RunnerOptions{
			PacketConn: pc,
			Handler:    dnssrv.HandlerFunc(func(dnssrv.ResponseWriter, *dnssrv.Msg) {}),
		}, nil
	}

	runner := dns.NewRunner(newOpts, zaptest.NewLogger(t))

	for i := range 2 {
		ctx, cancel := context.WithCancel(t.Context())

		errCh := make(chan error, 1)
		go func() { errCh <- runner.Serve(ctx) }()

		// Give the server time to come up, then assert below that it did not
		// exit early (e.g. returning "already started" on the second start).
		time.Sleep(200 * time.Millisecond)

		select {
		case err := <-errCh:
			t.Fatalf("runner exited early on start %d: %v", i, err)
		default:
		}

		cancel()

		select {
		case err := <-errCh:
			if err != nil && strings.Contains(err.Error(), "already started") {
				t.Fatalf("runner restart failed on start %d: %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for runner to stop on start %d", i)
		}
	}
}

func newManager(t *testing.T, nameservers ...string) func() {
	m := dns.NewManager(
		&testMemberReader{},
		&testStaticHostReader{},
		func(e suture.Event) { t.Log("dns-runners event:", e) },
		zaptest.NewLogger(t),
	)

	m.AllowNodeResolving(true)

	t.Cleanup(func() {
		if err := m.ClearAll(false); err != nil {
			t.Logf("error stopping dns runners: %v", err)
		}
	})

	pxs := xslices.Map(nameservers, func(ns string) *proxy.Proxy {
		p := proxy.NewProxy(ns, net.JoinHostPort(ns, "53"), "dns")
		p.Start(500 * time.Millisecond)

		return p
	})

	t.Cleanup(func() {
		for _, p := range pxs {
			p.Close() // We had to manually add this method to the coredns Proxy type.
		}
	})

	ctx, cancel := context.WithCancel(context.Background()) //nolint:usetesting
	t.Cleanup(cancel)

	m.SetUpstreams(xiter.Map(func(p *proxy.Proxy) dns.Upstream { return p }, slices.Values(pxs)))

	m.ServeBackground(ctx)
	m.ServeBackground(ctx)

	for _, err := range m.RunAll(slices.Values([]dns.AddressPair{
		{Network: "udp", Addr: netip.MustParseAddrPort("127.0.0.1:10700")},
		{Network: "udp", Addr: netip.MustParseAddrPort("127.0.0.1:10701")},
		{Network: "tcp", Addr: netip.MustParseAddrPort("127.0.0.1:10700")},
		{Network: "udp", Addr: netip.MustParseAddrPort("[::1]:10700")},
		{Network: "tcp", Addr: netip.MustParseAddrPort("[::1]:10700")},
		{Network: "udp", Addr: netip.MustParseAddrPort("[::1]:10700")},
	}), false) {
		if err != nil && strings.Contains(err.Error(), "failed to set TCP_FASTOPEN") {
			continue
		}

		require.NoError(t, err)
	}

	for _, err := range m.RunAll(slices.Values([]dns.AddressPair{
		{Network: "udp", Addr: netip.MustParseAddrPort("127.0.0.1:10700")},
		{Network: "tcp", Addr: netip.MustParseAddrPort("127.0.0.1:10700")},
		{Network: "udp", Addr: netip.MustParseAddrPort("[::1]:10700")},
		{Network: "tcp", Addr: netip.MustParseAddrPort("[::1]:10700")},
	}), false) {
		if err != nil && strings.Contains(err.Error(), "failed to set TCP_FASTOPEN") {
			continue
		}

		require.NoError(t, err)
	}

	return func() {
		if err := m.ClearAll(false); err != nil {
			t.Logf("error stopping dns runners: %v", err)
		}
	}
}

func createQuery(name string) *dnssrv.Msg {
	return &dnssrv.Msg{
		MsgHdr: dnssrv.MsgHdr{
			Id:               dnssrv.Id(),
			RecursionDesired: true,
		},
		Question: []dnssrv.Question{
			{
				Name:   dnssrv.Fqdn(name),
				Qtype:  dnssrv.TypeA,
				Qclass: dnssrv.ClassINET,
			},
		},
	}
}

type testMemberReader struct{}

func (r *testMemberReader) ReadMembers(context.Context) (iter.Seq[*cluster.Member], error) {
	namesToAddresses := map[string][]netip.Addr{
		"talos-default-controlplane-1": {netip.MustParseAddr("172.20.0.2")},
		"talos-default-worker-1":       {netip.MustParseAddr("172.20.0.3")},
	}

	result := maps.ToSlice(namesToAddresses, func(k string, v []netip.Addr) *cluster.Member {
		result := cluster.NewMember(cluster.NamespaceName, k)

		result.TypedSpec().Addresses = v

		return result
	})

	return slices.Values(result), nil
}

type testStaticHostReader struct{}

func (r *testStaticHostReader) ReadStaticHosts(ctx context.Context, name string) (iter.Seq[netip.Addr], error) {
	switch name {
	case "static-host-1":
		return slices.Values([]netip.Addr{
			netip.MustParseAddr("10.1.0.1"),
			netip.MustParseAddr("ff00::1"),
		}), nil
	case "static-host-2":
		return slices.Values([]netip.Addr{
			netip.MustParseAddr("10.1.0.2"),
			netip.MustParseAddr("ff00::2"),
		}), nil
	default:
		return nil, errors.New("not found")
	}
}
