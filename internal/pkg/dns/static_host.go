// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"slices"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

// StaticHostHandler resolves a configured set of host names to fixed addresses.
//
// It sits in the handler chain ahead of NodeHandler/Handler so that statically
// configured entries always win against upstream resolution. On miss, the
// request is forwarded to the next handler.
type StaticHostHandler struct {
	next   plugin.Handler
	logger *zap.Logger

	mapper HostMapper
}

// NewStaticHostHandler creates a new StaticHostHandler with an empty table.
func NewStaticHostHandler(next plugin.Handler, mapper HostMapper, logger *zap.Logger) *StaticHostHandler {
	return &StaticHostHandler{next: next, logger: logger, mapper: mapper}
}

// Name implements plugin.Handler.
func (h *StaticHostHandler) Name() string {
	return "StaticHostHandler"
}

// ServeDNS implements plugin.Handler.
func (h *StaticHostHandler) ServeDNS(ctx context.Context, wrt dns.ResponseWriter, msg *dns.Msg) (int, error) {
	idx := slices.IndexFunc(msg.Question, func(q dns.Question) bool { return q.Qtype == dns.TypeA || q.Qtype == dns.TypeAAAA })
	if idx != 0 {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	req := request.Request{W: wrt, Req: msg}

	addrs, ok := h.mapper.ResolveAddr(ctx, req.QType(), req.Name())
	if !ok {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	answers := mapAnswers(addrs, req.Name())
	if len(answers) == 0 {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	resp := new(dns.Msg).SetReply(req.Req)
	resp.Authoritative = true
	resp.Answer = answers

	if err := wrt.WriteMsg(resp); err != nil {
		h.logger.Warn("error writing dns response in static host handler", zap.Error(err))
	}

	return dns.RcodeSuccess, nil
}
