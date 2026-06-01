// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"iter"
	"net/netip"
	"slices"
	"sync/atomic"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

const staticResponseTTL = 10

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

// NewNodeHandler creates a new NodeHandler.
func NewNodeHandler(next plugin.Handler, hostMapper HostMapper, logger *zap.Logger) *NodeHandler {
	return &NodeHandler{next: next, mapper: hostMapper, logger: logger}
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
	if idx != 0 {
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

// SetEnabled sets the handler enabled state.
func (h *NodeHandler) SetEnabled(enabled bool) {
	h.enabled.Store(enabled)
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
					Ttl:    staticResponseTTL,
				},
				A: addr.AsSlice(),
			})

		case addr.Is6():
			result = append(result, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    staticResponseTTL,
				},
				AAAA: addr.AsSlice(),
			})
		}
	}

	return result
}
