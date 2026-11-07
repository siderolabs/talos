// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"errors"
	"iter"
	"sync"

	"github.com/miekg/dns"
	"github.com/siderolabs/gen/xiter"
	"go.uber.org/zap"
)

// Upstream is the abstraction used by [Handler] to forward DNS queries to a
// concrete upstream resolver.
//
// It is implemented by the proxy package (used for plain DNS and DoT)
// and by the doh package, so the handler can iterate uniformly
// across all configured upstream protocols.
type Upstream interface {
	// Exchange sends a DNS query to the upstream and returns the response.
	//
	// The proto ("udp" or "tcp") is the protocol the query was received over: plain DNS
	// upstreams use it to pick the transport (retrying over TCP if the UDP response is truncated),
	// other upstreams ignore it.
	Exchange(ctx context.Context, req *dns.Msg, proto string) (*dns.Msg, error)
	// Addr returns the upstream address (used for logging).
	Addr() string
	// Fails returns the number of failed health checks since the last successful one.
	Fails() uint32
	// Healthcheck kicks off a round of health checks for this upstream.
	Healthcheck()
}

// Handler is a dns proxy selector.
type Handler struct {
	mx     sync.RWMutex
	dests  iter.Seq[Upstream]
	logger *zap.Logger
}

// NewHandler creates a new Handler.
func NewHandler(logger *zap.Logger) *Handler {
	return &Handler{
		logger: logger,
		dests:  xiter.Empty[Upstream],
	}
}

// ServeDNS implements [ChainHandler].
//
//nolint:gocyclo
func (h *Handler) ServeDNS(ctx context.Context, wrt dns.ResponseWriter, msg *dns.Msg) (int, error) {
	h.mx.RLock()
	defer h.mx.RUnlock()

	clientProto := requestProto(wrt)
	logger := h.logger.With(
		zap.Stringer("data", msg),
		zap.String("proto", clientProto),
		zap.String("question", requestQuestion(msg).Name),
		zap.Stringer("local_addr", wrt.LocalAddr()),
		zap.Stringer("remote_addr", wrt.RemoteAddr()),
	)

	var (
		called bool
		resp   *dns.Msg
		err    error
	)

	for _, ups := range orderUpstreams(h.dests) {
		called = true

		logger.Debug("making dns request", zap.String("upstream", ups.Addr()))

		resp, err = ups.Exchange(ctx, msg, clientProto)
		if err != nil {
			// Kick off health check to see if the upstream is broken, so that
			// further requests try healthy upstreams first.
			ups.Healthcheck()
		}

		if resp != nil && (resp.Rcode == dns.RcodeServerFailure || resp.Rcode == dns.RcodeRefused) {
			continue
		}

		if ctx.Err() != nil || err == nil {
			break
		}

		continue
	}

	if !called {
		return dns.RcodeServerFailure, errors.New("no destination available")
	}

	if ctx.Err() != nil {
		return dns.RcodeServerFailure, ctx.Err()
	} else if err != nil {
		return dns.RcodeServerFailure, err
	}

	if !matchResponse(msg, resp) {
		logger.Warn("dns response didn't match", zap.Stringer("data", resp))

		return dns.RcodeFormatError, nil
	}

	err = wrt.WriteMsg(resp)
	if err != nil {
		// We can't do much here, but at least log the error.
		logger.Warn("error writing dns response", zap.Error(err))
	}

	logger.Debug("dns response", zap.Stringer("data", resp))

	return dns.RcodeSuccess, nil
}

// orderUpstreams returns healthy upstreams first, preserving the configured order within each group.
//
// Unhealthy upstreams are still tried as a last resort, as the health check might fail
// while the upstream is still able to answer the actual queries.
func orderUpstreams(dests iter.Seq[Upstream]) []Upstream {
	var healthy, unhealthy []Upstream

	for ups := range dests {
		if ups.Fails() > 0 {
			unhealthy = append(unhealthy, ups)
		} else {
			healthy = append(healthy, ups)
		}
	}

	return append(healthy, unhealthy...)
}

// SetProxy sets destination dns proxy servers.
func (h *Handler) SetProxy(prxs iter.Seq[Upstream]) bool {
	h.mx.Lock()
	defer h.mx.Unlock()

	if xiter.Equal(h.dests, prxs) {
		return false
	}

	h.dests = prxs

	return true
}

// Stop stops and clears dns proxy selector.
func (h *Handler) Stop() { h.SetProxy(xiter.Empty[Upstream]) }
