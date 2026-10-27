// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns_test

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/request"
	dnssrv "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/pkg/dns"
	"github.com/siderolabs/talos/internal/pkg/dns/doh"
)

var (
	_ dns.Upstream = (*proxy.Proxy)(nil)
	_ dns.Upstream = (*doh.Proxy)(nil)
)

func TestHandlerPrefersHealthyUpstreams(t *testing.T) {
	t.Parallel()

	var order []string

	dead := &fakeUpstream{addr: "dead", order: &order, err: errors.New("i/o timeout")}
	good := &fakeUpstream{addr: "good", order: &order}

	h := dns.NewHandler(zaptest.NewLogger(t))
	h.SetProxy(slices.Values([]dns.Upstream{dead, good}))

	query := func(t *testing.T, expectedOrder ...string) {
		t.Helper()

		order = order[:0]

		w := &fakeResponseWriter{}

		code, err := h.ServeDNS(t.Context(), w, createQuery("example.com"))
		require.NoError(t, err)
		assert.Equal(t, dnssrv.RcodeSuccess, code)
		require.NotNil(t, w.msg)
		assert.Equal(t, expectedOrder, order)
	}

	// the dead upstream is tried first, as it's not known to be unhealthy yet
	query(t, "dead", "good")
	assert.EqualValues(t, 1, dead.healthchecks.Load())
	assert.EqualValues(t, 0, good.healthchecks.Load())

	// the dead upstream failed the health check, so it's not tried anymore while the other one works
	query(t, "good")
	query(t, "good")

	// the dead upstream recovers and gets back to its place
	dead.err = nil
	dead.fails.Store(0)

	query(t, "dead")

	// all upstreams are unhealthy, so they are still tried in the configured order
	dead.fails.Store(1)
	good.fails.Store(1)

	query(t, "dead")
}

type fakeUpstream struct {
	err   error
	order *[]string
	addr  string

	fails        atomic.Uint32
	healthchecks atomic.Int32
}

func (f *fakeUpstream) Connect(_ context.Context, state request.Request, _ proxy.Options) (*dnssrv.Msg, net.Addr, string, error) {
	*f.order = append(*f.order, f.addr)

	if f.err != nil {
		return nil, nil, "", f.err
	}

	resp := new(dnssrv.Msg)
	resp.SetReply(state.Req)

	return resp, nil, "udp", nil
}

func (f *fakeUpstream) Addr() string { return f.addr }

func (f *fakeUpstream) Fails() uint32 { return f.fails.Load() }

// Healthcheck simulates a health check which fails if the upstream is broken.
func (f *fakeUpstream) Healthcheck() {
	f.healthchecks.Add(1)

	if f.err != nil {
		f.fails.Add(1)
	}
}

type fakeResponseWriter struct {
	msg *dnssrv.Msg
}

func (w *fakeResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 53), Port: 53}
}

func (w *fakeResponseWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (w *fakeResponseWriter) WriteMsg(msg *dnssrv.Msg) error {
	w.msg = msg

	return nil
}

func (w *fakeResponseWriter) Write([]byte) (int, error) { return 0, errors.ErrUnsupported }
func (w *fakeResponseWriter) Close() error              { return nil }
func (w *fakeResponseWriter) TsigStatus() error         { return nil }
func (w *fakeResponseWriter) TsigTimersOnly(bool)       {}
func (w *fakeResponseWriter) Hijack()                   {}
