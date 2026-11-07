// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"strings"
	"time"

	"github.com/miekg/dns"
)

// responseType classifies the responses for caching.
type responseType int

const (
	responseNoError responseType = iota
	responseNameError
	responseServerError
	responseNoData
	responseDelegation
	responseMeta
	responseUpdate
	responseOtherError
)

// classifyResponse returns the type of the response.
func classifyResponse(m *dns.Msg, now time.Time) responseType {
	rt := typifyResponse(m, now)

	if rt == responseNoError && hasSOA(m) && isNoData(m) {
		return responseNoData
	}

	return rt
}

func typifyResponse(m *dns.Msg, now time.Time) responseType {
	if rt, ok := typifySpecial(m, now); ok {
		return rt
	}

	return typifyByRcode(m)
}

// typifySpecial classifies the responses which are not regular query responses.
func typifySpecial(m *dns.Msg, now time.Time) (responseType, bool) {
	switch m.Opcode {
	case dns.OpcodeUpdate:
		return responseUpdate, true
	case dns.OpcodeNotify:
		return responseMeta, true
	}

	if len(m.Question) > 0 && (m.Question[0].Qtype == dns.TypeAXFR || m.Question[0].Qtype == dns.TypeIXFR) {
		return responseMeta, true
	}

	if opt := m.IsEdns0(); opt != nil && opt.Do() && hasExpiredSignatures(m, now) {
		return responseOtherError, true
	}

	return 0, false
}

func typifyByRcode(m *dns.Msg) responseType {
	if len(m.Answer) > 0 {
		switch m.Rcode {
		case dns.RcodeSuccess:
			return responseNoError
		case dns.RcodeNameError:
			return responseOtherError
		}
	}

	switch m.Rcode {
	case dns.RcodeSuccess:
		switch {
		case hasSOA(m):
			return responseNoData
		case hasNS(m):
			return responseDelegation
		default:
			return responseNoError
		}
	case dns.RcodeNameError:
		return responseNameError
	case dns.RcodeServerFailure, dns.RcodeNotImplemented:
		return responseServerError
	default:
		return responseOtherError
	}
}

func hasNS(m *dns.Msg) bool {
	for _, rr := range m.Ns {
		if rr.Header().Rrtype == dns.TypeNS {
			return true
		}
	}

	return false
}

func hasExpiredSignatures(m *dns.Msg, now time.Time) bool {
	for _, section := range [][]dns.RR{m.Answer, m.Ns, m.Extra} {
		for _, rr := range section {
			if sig, ok := rr.(*dns.RRSIG); ok && !sig.ValidityPeriod(now) {
				return true
			}
		}
	}

	return false
}

// isCacheable returns true if the response of the given type can be cached.
func isCacheable(m *dns.Msg, rt responseType) bool {
	if m.Truncated || len(m.Question) == 0 {
		return false
	}

	switch rt {
	case responseOtherError, responseMeta, responseUpdate:
		return false
	case responseNameError:
		return hasSOA(m)
	case responseNoError:
		return hasSOA(m) || !isNoData(m)
	case responseServerError, responseNoData, responseDelegation:
		fallthrough
	default:
		return true
	}
}

// minimalTTL returns the minimal TTL of the records in the response, capped at maxTTL.
//
// For negative responses, the SOA MINIMUM field is taken into account as well (RFC 2308, section 5).
func minimalTTL(m *dns.Msg, rt responseType, maxTTL time.Duration) time.Duration {
	negative := rt == responseNameError || rt == responseNoData

	if rt != responseNoError && !negative {
		return defaultTTL
	}

	if !hasRecords(m) {
		return defaultTTL
	}

	ttl := maxTTL

	for _, section := range [][]dns.RR{m.Answer, m.Ns, m.Extra} {
		for _, rr := range section {
			switch rr := rr.(type) {
			case *dns.OPT:
				// the OPT record uses the TTL field for the extended rcode and flags
				continue
			case *dns.SOA:
				if negative {
					ttl = min(ttl, time.Duration(rr.Minttl)*time.Second)
				}
			}

			ttl = min(ttl, time.Duration(rr.Header().Ttl)*time.Second)
		}
	}

	return ttl
}

// hasRecords returns true if the response has any records (except the OPT record).
func hasRecords(m *dns.Msg) bool {
	if len(m.Answer)+len(m.Ns) > 0 {
		return true
	}

	return len(m.Extra) > 1 || (len(m.Extra) == 1 && m.Extra[0].Header().Rrtype != dns.TypeOPT)
}

func hasSOA(m *dns.Msg) bool {
	for _, rr := range m.Ns {
		if rr.Header().Rrtype == dns.TypeSOA {
			return true
		}
	}

	return false
}

// isNoData returns true if the response has answers, but none of them answers the question.
func isNoData(m *dns.Msg) bool {
	return len(m.Answer) > 0 && !answersQuestion(m)
}

// answersQuestion returns true if the answer section contains the record for the question,
// following the CNAME chain.
func answersQuestion(m *dns.Msg) bool {
	if m.Rcode != dns.RcodeSuccess || len(m.Question) == 0 || len(m.Answer) == 0 {
		return false
	}

	q := m.Question[0]

	switch q.Qtype {
	case dns.TypeANY:
		// any record for the name answers the question
		return hasRecord(m.Answer, q.Name, q.Qclass, func(uint16) bool { return true })
	case dns.TypeCNAME:
		target, ok := uniqueCNAMETarget(m.Answer, q.Name, q.Qclass)

		return ok && target != ""
	}

	name, ok := canonicalName(m.Answer, q.Name, q.Qclass)
	if !ok {
		return false
	}

	return hasRecord(m.Answer, name, q.Qclass, func(rrtype uint16) bool { return rrtype == q.Qtype })
}

// hasRecord returns true if there is a record for the name and class with the matching type.
func hasRecord(answer []dns.RR, name string, qclass uint16, typeMatches func(uint16) bool) bool {
	for _, rr := range answer {
		h := rr.Header()

		if typeMatches(h.Rrtype) && classMatches(h.Class, qclass) && strings.EqualFold(h.Name, name) {
			return true
		}
	}

	return false
}

func classMatches(rrClass, qclass uint16) bool {
	return qclass == dns.ClassANY || rrClass == qclass
}

// canonicalName follows the CNAME chain from the name, and returns the final name.
//
// It returns false if the chain is ambiguous or has a loop.
func canonicalName(answer []dns.RR, name string, qclass uint16) (string, bool) {
	visited := map[string]struct{}{}

	for {
		key := strings.ToLower(name)

		if _, loop := visited[key]; loop {
			return name, false
		}

		visited[key] = struct{}{}

		target, ok := uniqueCNAMETarget(answer, name, qclass)
		if !ok {
			return name, false
		}

		if target == "" {
			return name, true
		}

		name = target
	}
}

// uniqueCNAMETarget returns the target of the CNAME record for the owner, or "" if there is none.
//
// It returns false if there are several CNAME records with different targets.
func uniqueCNAMETarget(answer []dns.RR, owner string, qclass uint16) (string, bool) {
	var target string

	for _, rr := range answer {
		cname, ok := rr.(*dns.CNAME)
		if !ok || !classMatches(cname.Hdr.Class, qclass) || !strings.EqualFold(cname.Hdr.Name, owner) {
			continue
		}

		if target != "" && !strings.EqualFold(target, cname.Target) {
			return "", false
		}

		target = cname.Target
	}

	return target, true
}
