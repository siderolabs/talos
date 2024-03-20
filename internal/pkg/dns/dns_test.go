// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	dnssrv "github.com/miekg/dns"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/pkg/ctxutil"
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
			ctx, stop := newServer(t, test.nameservers...)

			stopOnce := sync.OnceFunc(stop)
			defer stopOnce()

			time.Sleep(10 * time.Millisecond)

			r, err := dnssrv.Exchange(createQuery(), "127.0.0.53:10700")
			test.errCheck(t, err)

			if r != nil {
				require.Equal(t, test.expectedCode, r.Rcode, r)
			}

			t.Logf("r: %s", r)

			stopOnce()

			<-ctx.Done()

			require.NoError(t, ctxutil.Cause(ctx))
		})
	}
}

func TestDNSEmptyDestinations(t *testing.T) {
	ctx, stop := newServer(t)

	stopOnce := sync.OnceFunc(stop)
	defer stopOnce()

	time.Sleep(10 * time.Millisecond)

	r, err := dnssrv.Exchange(createQuery(), "127.0.0.53:10700")
	require.NoError(t, err)
	require.Equal(t, dnssrv.RcodeServerFailure, r.Rcode, r)

	r, err = dnssrv.Exchange(createQuery(), "127.0.0.53:10700")
	require.NoError(t, err)
	require.Equal(t, dnssrv.RcodeServerFailure, r.Rcode, r)

	stopOnce()

	<-ctx.Done()

	require.NoError(t, ctxutil.Cause(ctx))
}

func newServer(t *testing.T, nameservers ...string) (context.Context, func()) {
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

	runner := dns.NewRunner(dns.NewServer(dns.ServerOptins{
		PacketConn: pc,
		Handler:    dns.NewCache(handler, l),
	}), l)

	return ctxutil.MonitorFn(context.Background(), runner.Run), runner.Stop
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

func TestActivateFailure(t *testing.T) {
	// Ensure that we correctly handle an error inside [dns.Runner.Run].
	l := zaptest.NewLogger(t)

	runner := dns.NewRunner(&testServer{t: t}, l)

	ctx := ctxutil.MonitorFn(context.Background(), runner.Run)
	defer runner.Stop()

	<-ctx.Done()

	require.Equal(t, errFailed, ctxutil.Cause(ctx))
}

func TestRunnerStopsBeforeRun(t *testing.T) {
	// Ensure that we correctly handle an error inside [dns.Runner.Run].
	l := zap.NewNop()

	for range 1000 {
		runner := dns.NewRunner(&runnerStopper{}, l)

		ctx := ctxutil.MonitorFn(context.Background(), runner.Run)
		runner.Stop()

		<-ctx.Done()
	}

	for range 1000 {
		runner := dns.NewRunner(&runnerStopper{}, l)

		runner.Stop()
		ctx := ctxutil.MonitorFn(context.Background(), runner.Run)

		<-ctx.Done()
	}
}

type testServer struct {
	t *testing.T
}

var errFailed = errors.New("listen failure")

func (ts *testServer) ActivateAndServe() error { return errFailed }

func (ts *testServer) Shutdown() error {
	ts.t.Fatal("should not be called")

	return nil
}

func (ts *testServer) Name() string {
	return "test-server"
}

type runnerStopper struct {
	val atomic.Pointer[chan struct{}]
}

func (rs *runnerStopper) ActivateAndServe() error {
	ch := make(chan struct{})

	if rs.val.Swap(&ch) != nil {
		panic("chan should be empty")
	}

	<-ch

	return nil
}

func (rs *runnerStopper) Shutdown() error {
	chPtr := rs.val.Load()

	if chPtr == nil {
		return errors.New("server not started")
	}

	close(*chPtr)

	return nil
}
