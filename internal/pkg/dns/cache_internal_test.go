// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// The cache tests are in the package to control the clock.

// upstreamFunc is a [ChainHandler] which counts the calls.
type upstreamFunc struct {
	fn    func(w dns.ResponseWriter, req *dns.Msg) (int, error)
	calls int
	last  *dns.Msg
}

func (u *upstreamFunc) ServeDNS(_ context.Context, w dns.ResponseWriter, req *dns.Msg) (int, error) {
	u.calls++
	u.last = req

	return u.fn(w, req)
}

// answer returns an upstream answering with the records.
func answer(rcode int, build func(req, resp *dns.Msg)) *upstreamFunc {
	return &upstreamFunc{fn: func(w dns.ResponseWriter, req *dns.Msg) (int, error) {
		resp := new(dns.Msg)
		resp.SetRcode(req, rcode)
		resp.RecursionAvailable = true

		if build != nil {
			build(req, resp)
		}

		return dns.RcodeSuccess, w.WriteMsg(resp)
	}}
}

func aRecord(name string, ttl uint32) dns.RR {
	return &dns.A{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl}, A: net.IPv4(10, 0, 0, 1)}
}

func soaRecord(ttl uint32) dns.RR {
	return &dns.SOA{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: ttl},
		Ns:  "ns.example.com.", Mbox: "admin.example.com.", Minttl: ttl,
	}
}

type testWriter struct {
	remote net.Addr
	msgs   []*dns.Msg
}

func (w *testWriter) LocalAddr() net.Addr { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 53), Port: 53} }

func (w *testWriter) RemoteAddr() net.Addr {
	if w.remote != nil {
		return w.remote
	}

	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (w *testWriter) WriteMsg(msg *dns.Msg) error {
	w.msgs = append(w.msgs, msg)

	return nil
}

func (w *testWriter) Write([]byte) (int, error) { return 0, errors.ErrUnsupported }
func (w *testWriter) Close() error              { return nil }
func (w *testWriter) TsigStatus() error         { return nil }
func (w *testWriter) TsigTimersOnly(bool)       {}
func (w *testWriter) Hijack()                   {}

type cacheTest struct {
	cache *Cache
	now   time.Time
}

func newCacheTest(t *testing.T, next ChainHandler) *cacheTest {
	ct := &cacheTest{now: time.Unix(1_700_000_000, 0)}

	ct.cache = NewCache(next, zaptest.NewLogger(t))
	ct.cache.now = func() time.Time { return ct.now }

	return ct
}

func (ct *cacheTest) query(t *testing.T, name string, qtype uint16, modify ...func(*dns.Msg)) *dns.Msg {
	t.Helper()

	req := new(dns.Msg)
	req.SetQuestion(name, qtype)

	for _, m := range modify {
		m(req)
	}

	w := &testWriter{}
	ct.cache.ServeDNS(w, req)

	require.Len(t, w.msgs, 1)

	resp := w.msgs[0]
	assert.Equal(t, req.Id, resp.Id)

	return resp
}

func TestCachePositive(t *testing.T) {
	upstream := answer(dns.RcodeSuccess, func(req, resp *dns.Msg) {
		resp.Answer = append(resp.Answer, aRecord(req.Question[0].Name, 60))
	})

	ct := newCacheTest(t, upstream)

	resp := ct.query(t, "example.com.", dns.TypeA)
	require.Len(t, resp.Answer, 1)
	assert.EqualValues(t, 60, resp.Answer[0].Header().Ttl)
	assert.True(t, resp.RecursionAvailable)
	assert.Equal(t, 1, upstream.calls)

	// served from the cache with the remaining TTL
	ct.now = ct.now.Add(20 * time.Second)

	resp = ct.query(t, "example.com.", dns.TypeA)
	require.Len(t, resp.Answer, 1)
	assert.EqualValues(t, 40, resp.Answer[0].Header().Ttl)
	assert.True(t, resp.RecursionAvailable)
	assert.Equal(t, 1, upstream.calls)

	// the name is case-insensitive, but the response keeps the question as asked
	resp = ct.query(t, "EXAMPLE.com.", dns.TypeA)
	assert.Equal(t, "EXAMPLE.com.", resp.Question[0].Name)
	assert.Equal(t, 1, upstream.calls)

	// different type is a different entry
	ct.query(t, "example.com.", dns.TypeAAAA)
	assert.Equal(t, 2, upstream.calls)

	// different DO bit is a different entry
	ct.query(t, "example.com.", dns.TypeA, func(m *dns.Msg) { m.SetEdns0(1232, true) })
	assert.Equal(t, 3, upstream.calls)

	// after expiration, the upstream is asked again
	ct.now = ct.now.Add(41 * time.Second)

	resp = ct.query(t, "example.com.", dns.TypeA)
	assert.EqualValues(t, 60, resp.Answer[0].Header().Ttl)
	assert.Equal(t, 4, upstream.calls)

	// clearing the cache
	ct.cache.Clear()
	ct.query(t, "example.com.", dns.TypeA)
	assert.Equal(t, 5, upstream.calls)
}

func TestCacheTTLBounds(t *testing.T) {
	for _, test := range []struct {
		name        string
		recordTTL   uint32
		expectedTTL uint32
	}{
		{name: "min", recordTTL: 1, expectedTTL: 5},
		{name: "max", recordTTL: 86400, expectedTTL: 3600},
	} {
		t.Run(test.name, func(t *testing.T) {
			upstream := answer(dns.RcodeSuccess, func(req, resp *dns.Msg) {
				resp.Answer = append(resp.Answer, aRecord(req.Question[0].Name, test.recordTTL))
			})

			ct := newCacheTest(t, upstream)

			resp := ct.query(t, "example.com.", dns.TypeA)
			assert.EqualValues(t, test.expectedTTL, resp.Answer[0].Header().Ttl)

			ct.now = ct.now.Add(time.Duration(test.expectedTTL-1) * time.Second)

			resp = ct.query(t, "example.com.", dns.TypeA)
			assert.EqualValues(t, 1, resp.Answer[0].Header().Ttl)
			assert.Equal(t, 1, upstream.calls)
		})
	}
}

func TestCacheNegative(t *testing.T) {
	t.Run("NXDOMAIN with SOA", func(t *testing.T) {
		upstream := answer(dns.RcodeNameError, func(_, resp *dns.Msg) {
			resp.Ns = append(resp.Ns, soaRecord(300))
		})

		ct := newCacheTest(t, upstream)

		resp := ct.query(t, "missing.example.com.", dns.TypeA)
		assert.Equal(t, dns.RcodeNameError, resp.Rcode)
		// negative TTL is capped at 10 seconds
		assert.EqualValues(t, 10, resp.Ns[0].Header().Ttl)

		ct.now = ct.now.Add(9 * time.Second)

		resp = ct.query(t, "missing.example.com.", dns.TypeA)
		assert.Equal(t, dns.RcodeNameError, resp.Rcode)
		assert.Equal(t, 1, upstream.calls)

		ct.now = ct.now.Add(2 * time.Second)

		ct.query(t, "missing.example.com.", dns.TypeA)
		assert.Equal(t, 2, upstream.calls)
	})

	t.Run("NXDOMAIN with SOA minimum", func(t *testing.T) {
		upstream := answer(dns.RcodeNameError, func(_, resp *dns.Msg) {
			soa := soaRecord(300)
			soa.(*dns.SOA).Minttl = 7 //nolint:errcheck,forcetypeassert

			resp.Ns = append(resp.Ns, soa)
		})

		ct := newCacheTest(t, upstream)

		// negative TTL is the minimum of the SOA TTL and SOA MINIMUM
		resp := ct.query(t, "missing.example.com.", dns.TypeA)
		assert.EqualValues(t, 7, resp.Ns[0].Header().Ttl)

		ct.now = ct.now.Add(6 * time.Second)

		ct.query(t, "missing.example.com.", dns.TypeA)
		assert.Equal(t, 1, upstream.calls)

		ct.now = ct.now.Add(2 * time.Second)

		ct.query(t, "missing.example.com.", dns.TypeA)
		assert.Equal(t, 2, upstream.calls)
	})

	t.Run("NXDOMAIN without SOA", func(t *testing.T) {
		upstream := answer(dns.RcodeNameError, nil)

		ct := newCacheTest(t, upstream)

		ct.query(t, "missing.example.com.", dns.TypeA)
		ct.query(t, "missing.example.com.", dns.TypeA)
		assert.Equal(t, 2, upstream.calls)
	})

	t.Run("NODATA", func(t *testing.T) {
		upstream := answer(dns.RcodeSuccess, func(_, resp *dns.Msg) {
			resp.Ns = append(resp.Ns, soaRecord(1))
		})

		ct := newCacheTest(t, upstream)

		resp := ct.query(t, "example.com.", dns.TypeAAAA)
		assert.Empty(t, resp.Answer)
		// negative TTL is at least 5 seconds
		assert.EqualValues(t, 5, resp.Ns[0].Header().Ttl)

		ct.query(t, "example.com.", dns.TypeAAAA)
		assert.Equal(t, 1, upstream.calls)
	})
}

func TestCacheServerFailure(t *testing.T) {
	upstream := &upstreamFunc{fn: func(dns.ResponseWriter, *dns.Msg) (int, error) {
		return dns.RcodeServerFailure, errors.New("no upstream responded")
	}}

	ct := newCacheTest(t, upstream)

	resp := ct.query(t, "example.com.", dns.TypeA)
	assert.Equal(t, dns.RcodeServerFailure, resp.Rcode)
	assert.Equal(t, 1, upstream.calls)

	// the failure is cached for 5 seconds
	ct.now = ct.now.Add(4 * time.Second)

	resp = ct.query(t, "example.com.", dns.TypeA)
	assert.Equal(t, dns.RcodeServerFailure, resp.Rcode)
	assert.Equal(t, 1, upstream.calls)

	ct.now = ct.now.Add(2 * time.Second)

	ct.query(t, "example.com.", dns.TypeA)
	assert.Equal(t, 2, upstream.calls)
}

func TestCacheNotCached(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(req, resp *dns.Msg)
	}{
		{
			name: "truncated",
			build: func(req, resp *dns.Msg) {
				resp.Truncated = true
				resp.Answer = append(resp.Answer, aRecord(req.Question[0].Name, 60))
				// not cached: the upstream OPT record is dropped, as the client didn't send one
				resp.SetEdns0(1232, false)
			},
		},
		{
			name: "question mismatch",
			build: func(_, resp *dns.Msg) {
				resp.Question[0].Name = "other.example.com."
				resp.Answer = append(resp.Answer, aRecord("other.example.com.", 60))
			},
		},
		{
			name: "answer doesn't answer the question, no SOA",
			build: func(_, resp *dns.Msg) {
				resp.Answer = append(resp.Answer, aRecord("unrelated.example.com.", 60))
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			upstream := answer(dns.RcodeSuccess, test.build)

			ct := newCacheTest(t, upstream)

			resp := ct.query(t, "example.com.", dns.TypeA)
			assert.Nil(t, resp.IsEdns0())
			// TTLs of non-cached responses are passed through
			assert.EqualValues(t, 60, resp.Answer[0].Header().Ttl)

			ct.query(t, "example.com.", dns.TypeA)
			assert.Equal(t, 2, upstream.calls)
		})
	}
}

func TestSetResponseEDNS0(t *testing.T) {
	upstreamOpt := func() *dns.OPT {
		opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
		opt.SetUDPSize(1232)
		opt.SetDo()
		opt.SetVersion(1)
		opt.SetExtendedRcode(dns.RcodeBadVers)
		opt.Option = append(opt.Option, &dns.EDNS0_COOKIE{Code: dns.EDNS0COOKIE, Cookie: "0123456789abcdef"})

		return opt
	}

	t.Run("no EDNS0 in the request", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)

		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Extra = append(resp.Extra, aRecord("example.com.", 60), upstreamOpt())

		setResponseEDNS0(req, resp)

		assert.Nil(t, resp.IsEdns0())
		assert.Len(t, resp.Extra, 1)
	})

	for _, test := range []struct {
		name string
		opt  *dns.OPT
	}{
		{name: "no OPT in the response"},
		{name: "OPT in the response", opt: upstreamOpt()},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := new(dns.Msg)
			req.SetQuestion("example.com.", dns.TypeA)
			req.SetEdns0(4096, true)

			resp := new(dns.Msg)
			resp.SetReply(req)

			if test.opt != nil {
				resp.Extra = append(resp.Extra, test.opt)
			}

			setResponseEDNS0(req, resp)

			require.Len(t, resp.Extra, 1)

			opt := resp.IsEdns0()
			require.NotNil(t, opt)

			assert.EqualValues(t, 4096, opt.UDPSize())
			assert.True(t, opt.Do())
			assert.Zero(t, opt.Version())
			assert.Zero(t, opt.ExtendedRcode())
			assert.Empty(t, opt.Option)
		})
	}
}

func TestCacheAuthenticatedData(t *testing.T) {
	upstream := answer(dns.RcodeSuccess, func(req, resp *dns.Msg) {
		resp.AuthenticatedData = req.AuthenticatedData
		resp.Answer = append(resp.Answer, aRecord(req.Question[0].Name, 60))
	})

	ct := newCacheTest(t, upstream)

	// the client doesn't ask for AD, but the upstream is asked for it
	resp := ct.query(t, "example.com.", dns.TypeA)
	assert.True(t, upstream.last.AuthenticatedData)
	assert.False(t, resp.AuthenticatedData)

	// the cached AD bit is returned to the clients asking for it
	resp = ct.query(t, "example.com.", dns.TypeA, func(m *dns.Msg) { m.AuthenticatedData = true })
	assert.True(t, resp.AuthenticatedData)

	resp = ct.query(t, "example.com.", dns.TypeA)
	assert.False(t, resp.AuthenticatedData)
	assert.Equal(t, 1, upstream.calls)
}

func TestCacheScrub(t *testing.T) {
	upstream := answer(dns.RcodeSuccess, func(req, resp *dns.Msg) {
		for range 100 {
			resp.Answer = append(resp.Answer, aRecord(req.Question[0].Name, 60))
		}
	})

	ct := newCacheTest(t, upstream)

	// plain UDP client: the response is truncated to 512 bytes
	resp := ct.query(t, "example.com.", dns.TypeA)
	assert.True(t, resp.Truncated)
	assert.LessOrEqual(t, resp.Len(), dns.MinMsgSize)
	assert.Nil(t, resp.IsEdns0())

	// EDNS0 client: the response fits into the advertised buffer, and has the OPT record
	resp = ct.query(t, "example.com.", dns.TypeA, func(m *dns.Msg) {
		m.SetEdns0(4096, true)
		m.IsEdns0().Option = append(m.IsEdns0().Option, &dns.EDNS0_COOKIE{Code: dns.EDNS0COOKIE, Cookie: "0123456789abcdef"})
	})
	assert.False(t, resp.Truncated)
	assert.Len(t, resp.Answer, 100)

	opt := resp.IsEdns0()
	require.NotNil(t, opt)
	assert.EqualValues(t, 4096, opt.UDPSize())
	assert.True(t, opt.Do())
	// EDNS0 options are not supported, so they are not echoed back
	assert.Empty(t, opt.Option)

	// TCP client: no truncation
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	w := &testWriter{remote: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}}
	ct.cache.ServeDNS(w, req)

	require.Len(t, w.msgs, 1)
	assert.False(t, w.msgs[0].Truncated)
	assert.Len(t, w.msgs[0].Answer, 100)
}

func TestCacheStoreCapacity(t *testing.T) {
	store := newCacheStore(2)

	for _, name := range []string{"a.", "b.", "c."} {
		store.add(cacheKey{name: name}, &cacheItem{})
	}

	assert.Len(t, store.items, 2)

	// replacing an existing item doesn't evict
	for key := range store.items {
		store.add(key, &cacheItem{})
	}

	assert.Len(t, store.items, 2)
}

func TestClassifyResponse(t *testing.T) {
	cname := func(name, target string) dns.RR {
		return &dns.CNAME{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60}, Target: target}
	}

	now := time.Now()

	for _, test := range []struct {
		name      string
		rcode     int
		answer    []dns.RR
		ns        []dns.RR
		expected  responseType
		cacheable bool
	}{
		{name: "answer", answer: []dns.RR{aRecord("example.com.", 60)}, expected: responseNoError, cacheable: true},
		{
			name:      "CNAME chain",
			answer:    []dns.RR{cname("example.com.", "a.example.net."), cname("a.example.net.", "b.example.org."), aRecord("b.example.org.", 60)},
			expected:  responseNoError,
			cacheable: true,
		},
		{
			name:      "CNAME chain without the final record, with SOA",
			answer:    []dns.RR{cname("example.com.", "a.example.net.")},
			ns:        []dns.RR{soaRecord(60)},
			expected:  responseNoData,
			cacheable: true,
		},
		{
			name:     "CNAME chain without the final record, no SOA",
			answer:   []dns.RR{cname("example.com.", "a.example.net.")},
			expected: responseNoError,
		},
		{
			name:     "CNAME loop",
			answer:   []dns.RR{cname("example.com.", "a.example.net."), cname("a.example.net.", "example.com.")},
			expected: responseNoError,
		},
		{name: "NODATA", ns: []dns.RR{soaRecord(60)}, expected: responseNoData, cacheable: true},
		{name: "NXDOMAIN", rcode: dns.RcodeNameError, ns: []dns.RR{soaRecord(60)}, expected: responseNameError, cacheable: true},
		{name: "NXDOMAIN with answers", rcode: dns.RcodeNameError, answer: []dns.RR{aRecord("example.com.", 60)}, expected: responseOtherError},
		{name: "SERVFAIL", rcode: dns.RcodeServerFailure, expected: responseServerError, cacheable: true},
		{name: "REFUSED", rcode: dns.RcodeRefused, expected: responseOtherError},
		{
			name: "delegation",
			ns: []dns.RR{&dns.NS{
				Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60},
				Ns:  "ns.example.com.",
			}},
			expected:  responseDelegation,
			cacheable: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := new(dns.Msg)
			req.SetQuestion("example.com.", dns.TypeA)

			resp := new(dns.Msg)
			resp.SetRcode(req, test.rcode)
			resp.Answer = test.answer
			resp.Ns = test.ns

			rt := classifyResponse(resp, now)
			assert.Equal(t, test.expected, rt)
			assert.Equal(t, test.cacheable, isCacheable(resp, rt))
		})
	}
}
