// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dns provides dns server implementation.
package dns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/cache"
	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

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

		return dns.RcodeFormatError, nil
	}

	err = wrt.WriteMsg(resp)
	if err != nil {
		// We can't do much here, but at least log the error.
		h.logger.Warn("error writing dns response", zap.Error(err))
	}

	h.logger.Debug("dns response", zap.Stringer("data", resp))

	return dns.RcodeSuccess, nil
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

// NewNodeHandler creates a new NodeHandler.
func NewNodeHandler(next plugin.Handler, hostMapper HostMapper, logger *zap.Logger) *NodeHandler {
	return &NodeHandler{next: next, mapper: hostMapper, logger: logger}
}

// HostMapper is a name to node mapper.
type HostMapper interface {
	ResolveAddr(ctx context.Context, qType uint16, name string) []netip.Addr
}

// NodeHandler try to resolve dns request to a node. If required node is not found, it will move to the next handler.
type NodeHandler struct {
	next   plugin.Handler
	mapper HostMapper
	logger *zap.Logger

	enabled atomic.Bool
}

// Name implements plugin.Handler.
func (h *NodeHandler) Name() string {
	return "NodeHandler"
}

// ServeDNS implements plugin.Handler.
func (h *NodeHandler) ServeDNS(ctx context.Context, wrt dns.ResponseWriter, msg *dns.Msg) (int, error) {
	if !h.enabled.Load() {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	idx := slices.IndexFunc(msg.Question, func(q dns.Question) bool { return q.Qtype == dns.TypeA || q.Qtype == dns.TypeAAAA })
	if idx == -1 {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	req := request.Request{W: wrt, Req: msg}

	// Check if the request is for a node.
	result := h.mapper.ResolveAddr(ctx, req.QType(), req.Name())
	if len(result) == 0 {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	resp := new(dns.Msg).SetReply(req.Req)
	resp.Authoritative = true
	resp.Answer = mapAnswers(result, req.Name())

	err := wrt.WriteMsg(resp)
	if err != nil {
		// We can't do much here, but at least log the error.
		h.logger.Warn("error writing dns response in node handler", zap.Error(err))
	}

	return dns.RcodeSuccess, nil
}

func mapAnswers(addrs []netip.Addr, name string) []dns.RR {
	var result []dns.RR

	for _, addr := range addrs {
		switch {
		case addr.Is4():
			result = append(result, &dns.A{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    nodeDNSResponseTTL,
				},
				A: addr.AsSlice(),
			})

		case addr.Is6():
			result = append(result, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    nodeDNSResponseTTL,
				},
				AAAA: addr.AsSlice(),
			})
		}
	}

	return result
}

const nodeDNSResponseTTL = 10

// SetEnabled sets the handler enabled state.
func (h *NodeHandler) SetEnabled(enabled bool) {
	h.enabled.Store(enabled)
}

// ServerOptions is a Server options.
type ServerOptions struct {
	Listener      net.Listener
	PacketConn    net.PacketConn
	Handler       dns.Handler
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	IdleTimeout   func() time.Duration
	MaxTCPQueries int
	Logger        *zap.Logger
}

// NewServer creates a new Server.
func NewServer(opts ServerOptions) *Server {
	return &Server{
		srv: &dns.Server{
			Listener:      opts.Listener,
			PacketConn:    opts.PacketConn,
			Handler:       opts.Handler,
			ReadTimeout:   opts.ReadTimeout,
			WriteTimeout:  opts.WriteTimeout,
			IdleTimeout:   opts.IdleTimeout,
			MaxTCPQueries: opts.MaxTCPQueries,
		},
		logger: opts.Logger,
	}
}

// Server is a dns server.
type Server struct {
	srv    *dns.Server
	logger *zap.Logger
}

// Start starts the dns server. Returns a function to stop the server.
func (s *Server) Start(onDone func(err error)) (stop func(), stopped <-chan struct{}) {
	done := make(chan struct{})

	fn := sync.OnceFunc(func() {
		for {
			err := s.srv.Shutdown()
			if err != nil {
				if strings.Contains(err.Error(), "server not started") {
					// There a possible scenario where `go func()` not yet reached `ActivateAndServe` and yielded CPU
					// time to another goroutine and then this closure reached `Shutdown`. In that case
					// `ActivateAndServe` will actually start after `Shutdown` and this closure will block forever
					// because `go func()` will never exit and close `done` channel.
					continue
				}

				s.logger.Error("error shutting down dns server", zap.Error(err))
			}

			break
		}

		closer := io.Closer(s.srv.Listener)
		if closer == nil {
			closer = s.srv.PacketConn
		}

		if closer != nil {
			err := closer.Close()
			if err != nil && !errors.Is(err, net.ErrClosed) {
				s.logger.Error("error closing dns server listener", zap.Error(err))
			} else {
				s.logger.Debug("dns server listener closed")
			}
		}

		<-done
	})

	go func() {
		defer close(done)

		onDone(s.srv.ActivateAndServe())
	}()

	return fn, done
}

// NewTCPListener creates a new TCP listener.
func NewTCPListener(network, addr string) (net.Listener, error) {
	var opts []controlOptions

	switch network {
	case "tcp", "tcp4":
		network = "tcp4"
		opts = tcpOptions

	case "tcp6":
		opts = tcpOptionsV6

	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	lc := net.ListenConfig{Control: makeControl(opts)}

	return lc.Listen(context.Background(), network, addr)
}

// NewUDPPacketConn creates a new UDP packet connection.
func NewUDPPacketConn(network, addr string) (net.PacketConn, error) {
	var opts []controlOptions

	switch network {
	case "udp", "udp4":
		network = "udp4"
		opts = udpOptions

	case "udp6":
		opts = udpOptionsV6

	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	lc := net.ListenConfig{
		Control: makeControl(opts),
	}

	return lc.ListenPacket(context.Background(), network, addr)
}

var (
	tcpOptions = []controlOptions{
		{unix.IPPROTO_IP, unix.IP_RECVTTL, 1, "failed to set IP_RECVTTL"},
		{unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 5, "failed to set TCP_FASTOPEN"}, // tcp specific stuff from systemd
		{unix.IPPROTO_TCP, unix.TCP_NODELAY, 1, "failed to set TCP_NODELAY"},   // tcp specific stuff from systemd
		{unix.IPPROTO_IP, unix.IP_TTL, 1, "failed to set IP_TTL"},
	}

	tcpOptionsV6 = []controlOptions{
		{unix.IPPROTO_IPV6, unix.IPV6_RECVHOPLIMIT, 1, "failed to set IPV6_RECVHOPLIMIT"},
		{unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 5, "failed to set TCP_FASTOPEN"}, // tcp specific stuff from systemd
		{unix.IPPROTO_TCP, unix.TCP_NODELAY, 1, "failed to set TCP_NODELAY"},   // tcp specific stuff from systemd
		{unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, 1, "failed to set IPV6_UNICAST_HOPS"},
	}

	udpOptions = []controlOptions{
		{unix.IPPROTO_IP, unix.IP_RECVTTL, 1, "failed to set IP_RECVTTL"},
		{unix.IPPROTO_IP, unix.IP_TTL, 1, "failed to set IP_TTL"},
	}

	udpOptionsV6 = []controlOptions{
		{unix.IPPROTO_IPV6, unix.IPV6_RECVHOPLIMIT, 1, "failed to set IPV6_RECVHOPLIMIT"},
		{unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, 1, "failed to set IPV6_UNICAST_HOPS"},
	}
)

type controlOptions struct {
	level        int
	opt          int
	val          int
	errorMessage string
}

func makeControl(opts []controlOptions) func(string, string, syscall.RawConn) error {
	return func(_ string, _ string, c syscall.RawConn) error {
		var resErr error

		err := c.Control(func(fd uintptr) {
			for _, opt := range opts {
				opErr := unix.SetsockoptInt(int(fd), opt.level, opt.opt, opt.val)
				if opErr != nil {
					resErr = fmt.Errorf(opt.errorMessage+": %w", opErr)

					return
				}
			}
		})
		if err != nil {
			return fmt.Errorf("failed in control call: %w", err)
		}

		if resErr != nil {
			return fmt.Errorf("failed to set socket options: %w", resErr)
		}

		return nil
	}
}
