// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"slices"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

// StaticHostHandler resolves a configured set of host names to fixed addresses.
//
// It sits in the handler chain ahead of NodeHandler/Handler so that statically
// configured entries always win against upstream resolution. On miss, the
// request is forwarded to the next handler.
type StaticHostHandler struct {
	next   ChainHandler
	logger *zap.Logger

	mapper HostMapper
}

// NewStaticHostHandler creates a new StaticHostHandler with an empty table.
func NewStaticHostHandler(next ChainHandler, mapper HostMapper, logger *zap.Logger) *StaticHostHandler {
	return &StaticHostHandler{next: next, logger: logger, mapper: mapper}
}

// ServeDNS implements [ChainHandler].
func (h *StaticHostHandler) ServeDNS(ctx context.Context, wrt dns.ResponseWriter, msg *dns.Msg) (int, error) {
	idx := slices.IndexFunc(msg.Question, func(q dns.Question) bool { return q.Qtype == dns.TypeA || q.Qtype == dns.TypeAAAA })
	if idx != 0 {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	name := requestName(msg)

	addrs, ok := h.mapper.ResolveAddr(ctx, msg.Question[0].Qtype, name)
	if !ok {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	answers := mapAnswers(addrs, name)
	if len(answers) == 0 {
		return h.next.ServeDNS(ctx, wrt, msg)
	}

	resp := new(dns.Msg).SetReply(msg)
	resp.Authoritative = true
	resp.Answer = answers

	if err := wrt.WriteMsg(resp); err != nil {
		h.logger.Warn("error writing dns response in static host handler", zap.Error(err))
	}

	return dns.RcodeSuccess, nil
}
