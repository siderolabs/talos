// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dns provides dns server implementation.
package dns

import (
	"context"
	"errors"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/cache"
	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/utils"
)

// NewRunner creates a new Runner.
func NewRunner(srv Server, logger *zap.Logger) *Runner {
	r := utils.NewRunner(srv.ListenAndServe, srv.Shutdown, func(err error) bool {
		// There a possible scenario where `Run` reached `ListenAndServe` and then yielded CPU time to another
		// goroutine and then `Stop` reached `Shutdown`. In that case `ListenAndServe` will actually start after
		// `Shutdown` and `Stop` method will forever block if we do not try again.
		return strings.Contains(err.Error(), "server not started")
	})

	return &Runner{r: r, logger: logger}
}

// Runner is a dns server handler.
type Runner struct {
	r      *utils.Runner
	logger *zap.Logger
}

// Server is a dns server.
type Server interface {
	ListenAndServe() error
	Shutdown() error
}

// Run runs dns server.
func (r *Runner) Run() error {
	r.logger.Info("starting dns server")

	err := r.r.Run()

	r.logger.Info("dns server stopped", zap.Error(err))

	return err
}

// Stop stops dns server. It's safe to call even if server is already stopped.
func (r *Runner) Stop() {
	err := r.r.Stop()
	if err != nil {
		r.logger.Warn("error shutting down dns server", zap.Error(err))
	}
}

// Cache is a [dns.Handler] to [plugin.Handler] adapter.
type Cache struct {
	cache  *cache.Cache
	logger *zap.Logger
}

// NewCache creates a new Cache.
func NewCache(next plugin.Handler, l *zap.Logger) *Cache {
	c := cache.NewCache("zones", "view")
	c.Next = next

	return &Cache{cache: c, logger: l}
}

// ServeDNS implements [dns.Handler].
func (c *Cache) ServeDNS(wr dns.ResponseWriter, msg *dns.Msg) {
	_, err := c.cache.ServeDNS(context.Background(), wr, msg)
	if err != nil {
		// we should probably call newProxy.Healthcheck() if there are too many errors
		c.logger.Warn("error serving dns request", zap.Error(err))
	}
}

// Handler is a dns proxy selector.
type Handler struct {
	mx     sync.RWMutex
	dests  []*proxy.Proxy
	logger *zap.Logger
}

// NewHandler creates a new Handler.
func NewHandler(logger *zap.Logger) *Handler {
	return &Handler{
		logger: logger,
	}
}

// Name implements plugin.Handler.
func (h *Handler) Name() string {
	return "Handler"
}

// ServeDNS implements plugin.Handler.
func (h *Handler) ServeDNS(ctx context.Context, wrt dns.ResponseWriter, msg *dns.Msg) (int, error) {
	h.mx.RLock()
	defer h.mx.RUnlock()

	req := request.Request{W: wrt, Req: msg}

	h.logger.Debug("dns request", zap.Stringer("data", msg))

	upstreams := slices.Clone(h.dests)

	if len(upstreams) == 0 {
		emptyProxyErr := new(dns.Msg).SetRcode(req.Req, dns.RcodeServerFailure)

		err := wrt.WriteMsg(emptyProxyErr)
		if err != nil {
			// We can't do much here, but at least log the error.
			h.logger.Warn("failed to write 'no destination available' error dns response", zap.Error(err))
		}

		return dns.RcodeServerFailure, errors.New("no destination available")
	}

	rand.Shuffle(len(upstreams), func(i, j int) { upstreams[i], upstreams[j] = upstreams[j], upstreams[i] })

	var (
		resp *dns.Msg
		err  error
	)

	for _, ups := range upstreams {
		resp, err = ups.Connect(ctx, req, proxy.Options{})
		if errors.Is(err, proxy.ErrCachedClosed) { // Remote side closed conn, can only happen with TCP.
			continue
		}

		if err == nil {
			break
		}

		continue
	}

	if err != nil {
		return dns.RcodeServerFailure, err
	}

	if !req.Match(resp) {
		resp = new(dns.Msg).SetRcode(req.Req, dns.RcodeFormatError)

		err = wrt.WriteMsg(resp)
		if err != nil {
			// We can't do much here, but at least log the error.
			h.logger.Warn("failed to write non-matched response", zap.Error(err))
		}

		h.logger.Warn("dns response didn't match", zap.Stringer("data", resp))

		return 0, nil
	}

	err = wrt.WriteMsg(resp)
	if err != nil {
		// We can't do much here, but at least log the error.
		h.logger.Warn("error writing dns response", zap.Error(err))
	}

	h.logger.Debug("dns response", zap.Stringer("data", resp))

	return 0, nil
}

// SetProxy sets destination dns proxy servers.
func (h *Handler) SetProxy(prxs []*proxy.Proxy) bool {
	h.mx.Lock()
	defer h.mx.Unlock()

	if slices.Equal(h.dests, prxs) {
		return false
	}

	h.dests = prxs

	return true
}

// Stop stops and clears dns proxy selector.
func (h *Handler) Stop() { h.SetProxy(nil) }

// ServerOptins is a Server options.
type ServerOptins struct {
	Addr          string
	Net           string
	Handler       dns.Handler
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	IdleTimeout   func() time.Duration
	MaxTCPQueries int
}

// NewServer creates a new Server.
func NewServer(opts ServerOptins) Server {
	return &server{&dns.Server{
		Addr:          opts.Addr,
		Net:           opts.Net,
		Handler:       opts.Handler,
		ReadTimeout:   opts.ReadTimeout,
		WriteTimeout:  opts.WriteTimeout,
		IdleTimeout:   opts.IdleTimeout,
		MaxTCPQueries: opts.MaxTCPQueries,
	}}
}

type server struct{ *dns.Server }
