// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"errors"
	"iter"
	"sync"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/siderolabs/gen/xiter"
	"go.uber.org/zap"
)

// Upstream is the abstraction used by [Handler] to forward DNS queries to a
// concrete upstream resolver.
//
// It is implemented by the CoreDNS plugin proxy (used for plain DNS and DoT)
// and by the DoH proxy in this package, so the handler can iterate uniformly
// across all configured upstream protocols.
type Upstream interface {
	// Connect sends a DNS query to the upstream and returns the response.
	Connect(ctx context.Context, state request.Request, opts proxy.Options) (*dns.Msg, error)
	// Addr returns the upstream address (used for logging).
	Addr() string
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

// Name implements plugin.Handler.
func (h *Handler) Name() string {
	return "Handler"
}

// ServeDNS implements plugin.Handler.
//
//nolint:gocyclo
func (h *Handler) ServeDNS(ctx context.Context, wrt dns.ResponseWriter, msg *dns.Msg) (int, error) {
	h.mx.RLock()
	defer h.mx.RUnlock()

	req := request.Request{W: wrt, Req: msg}
	logger := h.logger.With(
		zap.Stringer("data", msg),
		zap.String("proto", req.Proto()),
		zap.String("question", req.QName()),
		zap.Stringer("local_addr", wrt.LocalAddr()),
		zap.Stringer("remote_addr", wrt.RemoteAddr()),
	)

	var (
		called bool
		resp   *dns.Msg
		err    error
	)

	for ups := range h.dests {
		called = true
		opts := proxy.Options{}

		logger.Debug("making dns request", zap.String("upstream", ups.Addr()))

		for {
			resp, err = ups.Connect(ctx, req, opts)

			switch {
			case errors.Is(err, proxy.ErrCachedClosed): // Remote side closed conn, can only happen with TCP.
				continue
			case resp != nil && resp.Truncated && !opts.ForceTCP: // Retry with TCP if truncated
				opts.ForceTCP = true

				continue
			}

			break
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

	if !req.Match(resp) {
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
