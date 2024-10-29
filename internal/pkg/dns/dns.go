// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dns provides dns server implementation.
package dns

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/cache"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/siderolabs/gen/xiter"
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
	c := cache.NewCache(
		"zones",
		"view",
		cache.WithNegativeTTL(10*time.Second, dnsutil.MinimalDefaultTTL),
	)
	c.Next = next

	return &Cache{cache: c, logger: l}
}

// ServeDNS implements [dns.Handler].
func (c *Cache) ServeDNS(wr dns.ResponseWriter, msg *dns.Msg) {
	wr = request.NewScrubWriter(msg, wr)

	ctx, cancel := context.WithTimeout(context.Background(), 4500*time.Millisecond)
	defer cancel()

	code, err := c.cache.ServeDNS(ctx, wr, msg)
	if err != nil {
		// we should probably call newProxy.Healthcheck() if there are too many errors
		c.logger.Warn("error serving dns request", zap.Error(err))
	}

	if clientWrite(code) {
		return
	}

	// Something went wrong
	state := request.Request{W: wr, Req: msg}

	answer := new(dns.Msg)
	answer.SetRcode(msg, code)
	state.SizeAndDo(answer)

	err = wr.WriteMsg(answer)
	if err != nil {
		c.logger.Warn("error writing dns response", zap.Error(err))
	}
}

// clientWrite returns true if the response has been written to the client.
func clientWrite(rcode int) bool {
	switch rcode {
	case dns.RcodeServerFailure, dns.RcodeRefused, dns.RcodeFormatError, dns.RcodeNotImplemented:
		return false
	default:
		return true
	}
}

// Handler is a dns proxy selector.
type Handler struct {
	mx     sync.RWMutex
	dests  iter.Seq[*proxy.Proxy]
	logger *zap.Logger
}

// NewHandler creates a new Handler.
func NewHandler(logger *zap.Logger) *Handler {
	return &Handler{
		logger: logger,
		dests:  xiter.Empty[*proxy.Proxy],
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

	h.logger.Debug("dns request", zap.Stringer("data", msg))

	var (
		called bool
		resp   *dns.Msg
		err    error
	)

	for ups := range h.dests {
		called = true
		opts := proxy.Options{}

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
func (h *Handler) SetProxy(prxs iter.Seq[*proxy.Proxy]) bool {
	h.mx.Lock()
	defer h.mx.Unlock()

	if xiter.Equal(h.dests, prxs) {
		return false
	}

	h.dests = prxs

	return true
}

// Stop stops and clears dns proxy selector.
func (h *Handler) Stop() { h.SetProxy(xiter.Empty) }

// NewNodeHandler creates a new NodeHandler.
func NewNodeHandler(next plugin.Handler, hostMapper HostMapper, logger *zap.Logger) *NodeHandler {
	return &NodeHandler{next: next, mapper: hostMapper, logger: logger}
}

// HostMapper is a name to node mapper.
type HostMapper interface {
	ResolveAddr(ctx context.Context, qType uint16, name string) (iter.Seq[netip.Addr], bool)
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
	result, ok := h.mapper.ResolveAddr(ctx, req.QType(), req.Name())
	if !ok {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	answers := mapAnswers(result, req.Name())
	if len(answers) == 0 {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	resp := new(dns.Msg).SetReply(req.Req)
	resp.Authoritative = true
	resp.Answer = answers

	err := wrt.WriteMsg(resp)
	if err != nil {
		// We can't do much here, but at least log the error.
		h.logger.Warn("error writing dns response in node handler", zap.Error(err))
	}

	return dns.RcodeSuccess, nil
}

func mapAnswers(addrs iter.Seq[netip.Addr], name string) []dns.RR {
	var result []dns.RR

	for addr := range addrs {
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

// NewTCPListener creates a new TCP listener.
func NewTCPListener(network, addr string, control ControlFn) (net.Listener, error) {
	network, ok := networkNames[network]
	if !ok {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	lc := net.ListenConfig{Control: control}

	return lc.Listen(context.Background(), network, addr)
}

// NewUDPPacketConn creates a new UDP packet connection.
func NewUDPPacketConn(network, addr string, control ControlFn) (net.PacketConn, error) {
	network, ok := networkNames[network]
	if !ok {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	lc := net.ListenConfig{Control: control}

	return lc.ListenPacket(context.Background(), network, addr)
}

// ControlFn is an alias to [net.ListenConfig.Control] function.
type ControlFn = func(string, string, syscall.RawConn) error

// MakeControl creates a control function for setting socket options.
func MakeControl(network string, forwardEnabled bool) (ControlFn, error) {
	maxHops := 1

	if forwardEnabled {
		maxHops = 2
	}

	var options []controlOptions

	switch network {
	case "tcp", "tcp4":
		options = []controlOptions{
			{unix.IPPROTO_IP, unix.IP_RECVTTL, maxHops, "failed to set IP_RECVTTL"},
			{unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 5, "failed to set TCP_FASTOPEN"}, // tcp specific stuff from systemd
			{unix.IPPROTO_TCP, unix.TCP_NODELAY, 1, "failed to set TCP_NODELAY"},   // tcp specific stuff from systemd
			{unix.IPPROTO_IP, unix.IP_TTL, maxHops, "failed to set IP_TTL"},
		}
	case "tcp6":
		options = []controlOptions{
			{unix.IPPROTO_IPV6, unix.IPV6_RECVHOPLIMIT, maxHops, "failed to set IPV6_RECVHOPLIMIT"},
			{unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 5, "failed to set TCP_FASTOPEN"}, // tcp specific stuff from systemd
			{unix.IPPROTO_TCP, unix.TCP_NODELAY, 1, "failed to set TCP_NODELAY"},   // tcp specific stuff from systemd
			{unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, maxHops, "failed to set IPV6_UNICAST_HOPS"},
		}
	case "udp", "udp4":
		options = []controlOptions{
			{unix.IPPROTO_IP, unix.IP_RECVTTL, maxHops, "failed to set IP_RECVTTL"},
			{unix.IPPROTO_IP, unix.IP_TTL, maxHops, "failed to set IP_TTL"},
		}
	case "udp6":
		options = []controlOptions{
			{unix.IPPROTO_IPV6, unix.IPV6_RECVHOPLIMIT, maxHops, "failed to set IPV6_RECVHOPLIMIT"},
			{unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, maxHops, "failed to set IPV6_UNICAST_HOPS"},
		}
	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	return func(_ string, _ string, c syscall.RawConn) error {
		var resErr error

		err := c.Control(func(fd uintptr) {
			for _, opt := range options {
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
	}, nil
}

type controlOptions struct {
	level        int
	opt          int
	val          int
	errorMessage string
}

var networkNames = map[string]string{
	"tcp":  "tcp4",
	"tcp4": "tcp4",
	"tcp6": "tcp6",
	"udp":  "udp4",
	"udp4": "udp4",
	"udp6": "udp6",
}
