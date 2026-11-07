// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"net"
	"slices"
	"strings"

	"github.com/miekg/dns"
)

// ChainHandler is a handler in the DNS request processing chain.
//
// It returns the response code, and a response is written to the client only if the code is
// one of the success codes (see [clientWrite]). Otherwise, the caller writes the error response.
type ChainHandler interface {
	ServeDNS(ctx context.Context, w dns.ResponseWriter, req *dns.Msg) (int, error)
}

const (
	protoUDP = "udp"
	protoTCP = "tcp"
)

// requestProto returns the protocol ("udp" or "tcp") the request was received over.
func requestProto(w dns.ResponseWriter) string {
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		return protoTCP
	}

	return protoUDP
}

// requestName returns the lowercased name of the first question.
func requestName(req *dns.Msg) string {
	if len(req.Question) == 0 {
		return "."
	}

	return strings.ToLower(dns.Name(req.Question[0].Name).String())
}

// requestQuestion returns the first question of the request.
func requestQuestion(req *dns.Msg) dns.Question {
	if len(req.Question) == 0 {
		return dns.Question{Name: "."}
	}

	return req.Question[0]
}

// requestDo returns the DNSSEC OK bit of the request.
func requestDo(req *dns.Msg) bool {
	if opt := req.IsEdns0(); opt != nil {
		return opt.Do()
	}

	return false
}

// requestSize returns the maximum size of the response the client can receive.
func requestSize(req *dns.Msg, proto string) int {
	if proto == protoTCP {
		return dns.MaxMsgSize
	}

	var size uint16

	if opt := req.IsEdns0(); opt != nil {
		size = opt.UDPSize()
	}

	return int(max(size, dns.MinMsgSize))
}

// matchResponse returns true if the response is a response to the request's question.
func matchResponse(req, resp *dns.Msg) bool {
	if len(resp.Question) != 1 || !resp.Response {
		return false
	}

	q := requestQuestion(req)

	return strings.ToLower(resp.Question[0].Name) == requestName(req) &&
		resp.Question[0].Qtype == q.Qtype &&
		resp.Question[0].Qclass == q.Qclass
}

// scrubWriter makes the responses fit the client: it sets the EDNS0 options from the request,
// and truncates the response to the size the client can receive.
type scrubWriter struct {
	dns.ResponseWriter

	req *dns.Msg
}

// WriteMsg implements [dns.ResponseWriter].
func (w *scrubWriter) WriteMsg(resp *dns.Msg) error {
	setResponseEDNS0(w.req, resp)
	scrub(w.req, resp, w.ResponseWriter)

	return w.ResponseWriter.WriteMsg(resp)
}

// setResponseEDNS0 sets the EDNS0 OPT record of the response to match the request:
// the OPT record is added if the request has one, and removed otherwise (RFC 6891, section 7).
//
// No EDNS0 options are supported, so they are not passed to the client.
func setResponseEDNS0(req, resp *dns.Msg) {
	reqOpt := req.IsEdns0()
	if reqOpt == nil {
		resp.Extra = slices.DeleteFunc(resp.Extra, func(rr dns.RR) bool { return rr.Header().Rrtype == dns.TypeOPT })

		return
	}

	opt := resp.IsEdns0()
	if opt == nil {
		opt = &dns.OPT{}
		resp.Extra = append(resp.Extra, opt)
	}

	opt.Hdr.Name = "."
	opt.Hdr.Rrtype = dns.TypeOPT
	// clear the version and flags (the extended rcode is set from the message rcode on packing)
	opt.Hdr.Ttl = 0
	opt.Option = nil
	opt.SetUDPSize(reqOpt.UDPSize())

	if reqOpt.Do() {
		opt.SetDo()
	}
}

// scrub truncates the response to the size the client can receive.
func scrub(req, resp *dns.Msg, w dns.ResponseWriter) {
	proto := requestProto(w)

	resp.Truncate(requestSize(req, proto))

	if resp.Compress || proto != protoUDP {
		return
	}

	// enable compression for large UDP responses to avoid fragmentation
	var isIPv6 bool

	if addr, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		isIPv6 = addr.IP.To4() == nil
	}

	switch l := resp.Len(); {
	case !isIPv6 && l > 1480:
		resp.Compress = true
	case isIPv6 && l > 1220:
		resp.Compress = true
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
