// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

const requestTimeout = 4500 * time.Millisecond

// Cache settings.
const (
	// cacheCapacity is the maximum number of entries in each of the positive and negative caches.
	cacheCapacity = 10000

	minPositiveTTL = 5 * time.Second
	maxPositiveTTL = time.Hour

	minNegativeTTL = 5 * time.Second
	maxNegativeTTL = 10 * time.Second

	// serverFailureTTL is the time to cache a server failure (e.g. no upstream responded).
	serverFailureTTL = 5 * time.Second

	// defaultTTL is the TTL used for responses without records.
	defaultTTL = 5 * time.Second
)

// Cache is a caching [dns.Handler], which forwards cache misses to the next [ChainHandler].
type Cache struct {
	next   ChainHandler
	logger *zap.Logger
	now    func() time.Time

	positive cacheStore
	negative cacheStore
}

// NewCache creates a new Cache.
func NewCache(next ChainHandler, l *zap.Logger) *Cache {
	return &Cache{
		next:     next,
		logger:   l,
		now:      time.Now,
		positive: newCacheStore(cacheCapacity),
		negative: newCacheStore(cacheCapacity),
	}
}

// ServeDNS implements [dns.Handler].
func (c *Cache) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	w = &scrubWriter{ResponseWriter: w, req: req}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	code, err := c.serveDNS(ctx, w, req)
	if err != nil {
		c.logger.Warn("error serving dns request", zap.Error(err))
	}

	if clientWrite(code) {
		return
	}

	// something went wrong, write the error response
	answer := new(dns.Msg)
	answer.SetRcode(req, code)

	if err = w.WriteMsg(answer); err != nil {
		c.logger.Warn("error writing dns response", zap.Error(err))
	}
}

// Clear clears the cache.
func (c *Cache) Clear() {
	c.positive.clear()
	c.negative.clear()
}

func (c *Cache) serveDNS(ctx context.Context, w dns.ResponseWriter, req *dns.Msg) (int, error) {
	if req.Opcode != dns.OpcodeQuery {
		return c.next.ServeDNS(ctx, w, req)
	}

	key := newCacheKey(req)
	ad := req.AuthenticatedData
	now := c.now()

	if item := c.get(key, req, now); item != nil {
		// the response was (partially) written, so return success to avoid writing the error response on top of it
		return dns.RcodeSuccess, w.WriteMsg(item.toMsg(req, now, key.do, ad))
	}

	// the request might be modified down the chain, so pass a copy;
	// ask the upstream to return the AD bit, so that the cached response can be served to clients asking for it
	upstreamReq := req.Copy()
	upstreamReq.AuthenticatedData = true

	cw := &cacheWriter{
		ResponseWriter: w,
		cache:          c,
		req:            upstreamReq,
		key:            key,
		ad:             ad,
	}

	code, err := c.next.ServeDNS(ctx, cw, upstreamReq)
	if err != nil && code == dns.RcodeServerFailure && ctx.Err() == nil && !cw.written {
		// cache the failure for a short time, so that clients don't hammer broken upstreams
		failure := new(dns.Msg)
		failure.SetRcode(upstreamReq, dns.RcodeServerFailure)

		cw.store(failure)
	}

	return code, err
}

// get returns the cached item for the request, if it exists and has not expired.
func (c *Cache) get(key cacheKey, req *dns.Msg, now time.Time) *cacheItem {
	positive := c.positive.getFresh(key, req, now)

	if negative := c.negative.getFresh(key, req, now); negative != nil {
		// prefer a fresh positive response over a cached server failure
		if negative.rcode == dns.RcodeServerFailure && positive != nil {
			return positive
		}

		return negative
	}

	return positive
}

// cacheWriter caches the responses written by the next handler, and passes them to the client.
type cacheWriter struct {
	dns.ResponseWriter

	cache   *Cache
	req     *dns.Msg
	key     cacheKey
	ad      bool
	written bool
}

// WriteMsg implements [dns.ResponseWriter].
func (w *cacheWriter) WriteMsg(resp *dns.Msg) error {
	resp = w.store(resp)

	if !w.key.do && !w.ad {
		resp.AuthenticatedData = false
	}

	w.written = true

	return w.ResponseWriter.WriteMsg(resp)
}

// store caches a copy of the response (if it's cacheable), and returns the copy with the cache TTL set,
// so that the client sees the same TTL as on the following cache hits.
//
// Non-cacheable responses are returned as is.
func (w *cacheWriter) store(resp *dns.Msg) *dns.Msg {
	resp = resp.Copy()
	now := w.cache.now()
	rt := classifyResponse(resp, now)

	if !isCacheable(resp, rt) || !matchResponse(w.req, resp) {
		return resp
	}

	ttl := cacheTTL(resp, rt)
	ttlSeconds := uint32(ttl.Seconds())

	resp.Answer = setTTL(resp.Answer, ttlSeconds, false)
	resp.Ns = setTTL(resp.Ns, ttlSeconds, false)
	resp.Extra = setTTL(resp.Extra, ttlSeconds, false)

	switch rt {
	case responseNoError, responseDelegation:
		w.cache.positive.add(w.key, newCacheItem(resp, now, ttl))
	case responseNameError, responseNoData, responseServerError:
		w.cache.negative.add(w.key, newCacheItem(resp, now, ttl))
	case responseOtherError, responseMeta, responseUpdate:
	}

	return resp
}

// cacheTTL returns the time to cache the response for.
func cacheTTL(resp *dns.Msg, rt responseType) time.Duration {
	switch rt {
	case responseNameError, responseNoData:
		return min(max(minimalTTL(resp, rt, maxNegativeTTL), minNegativeTTL), maxNegativeTTL)
	case responseServerError:
		return serverFailureTTL
	case responseNoError, responseDelegation, responseMeta, responseUpdate, responseOtherError:
		fallthrough
	default:
		return min(max(minimalTTL(resp, rt, maxPositiveTTL), minPositiveTTL), maxPositiveTTL)
	}
}

// cacheKey identifies the cached response.
type cacheKey struct {
	name   string
	qtype  uint16
	qclass uint16
	do     bool
	cd     bool
}

func newCacheKey(req *dns.Msg) cacheKey {
	q := requestQuestion(req)

	return cacheKey{
		name:   requestName(req),
		qtype:  q.Qtype,
		qclass: q.Qclass,
		do:     requestDo(req),
		cd:     req.CheckingDisabled,
	}
}

// cacheItem is a cached response.
type cacheItem struct {
	stored time.Time

	name   string
	answer []dns.RR
	ns     []dns.RR
	extra  []dns.RR

	origTTL uint32
	rcode   int
	qtype   uint16
	qclass  uint16

	authoritative      bool
	authenticatedData  bool
	recursionAvailable bool
}

func newCacheItem(resp *dns.Msg, now time.Time, ttl time.Duration) *cacheItem {
	item := &cacheItem{
		rcode:              resp.Rcode,
		authoritative:      resp.Authoritative,
		authenticatedData:  resp.AuthenticatedData,
		recursionAvailable: resp.RecursionAvailable,
		answer:             resp.Answer,
		ns:                 resp.Ns,
		origTTL:            uint32(ttl.Seconds()),
		stored:             now,
	}

	if len(resp.Question) > 0 {
		item.name = resp.Question[0].Name
		item.qtype = resp.Question[0].Qtype
		item.qclass = resp.Question[0].Qclass
	}

	for _, rr := range resp.Extra {
		if rr.Header().Rrtype != dns.TypeOPT {
			item.extra = append(item.extra, rr)
		}
	}

	return item
}

// ttl returns the remaining TTL of the item in seconds.
func (i *cacheItem) ttl(now time.Time) int {
	return int(i.origTTL) - int(now.Sub(i.stored).Seconds())
}

func (i *cacheItem) matches(req *dns.Msg) bool {
	q := requestQuestion(req)

	return q.Qtype == i.qtype && q.Qclass == i.qclass && strings.EqualFold(q.Name, i.name)
}

// toMsg builds a response to the request from the cached item, with the remaining TTL.
func (i *cacheItem) toMsg(req *dns.Msg, now time.Time, do, ad bool) *dns.Msg {
	ttl := uint32(max(i.ttl(now), 0))

	resp := new(dns.Msg)
	resp.SetReply(req)

	resp.Authoritative = i.authoritative
	resp.AuthenticatedData = i.authenticatedData && (do || ad)
	resp.RecursionAvailable = i.recursionAvailable
	resp.Rcode = i.rcode

	resp.Answer = setTTL(i.answer, ttl, true)
	resp.Ns = setTTL(i.ns, ttl, true)
	resp.Extra = setTTL(i.extra, ttl, true)

	return resp
}

// setTTL drops OPT records and sets the TTL of the records, copying them if requested.
func setTTL(rrs []dns.RR, ttl uint32, copyRRs bool) []dns.RR {
	result := make([]dns.RR, 0, len(rrs))

	for _, rr := range rrs {
		if rr.Header().Rrtype == dns.TypeOPT {
			continue
		}

		if copyRRs {
			rr = dns.Copy(rr)
		}

		rr.Header().Ttl = ttl

		result = append(result, rr)
	}

	return result
}

// cacheStore is a fixed capacity store for cached items.
//
// When the store is full, a random item is evicted.
type cacheStore struct {
	items    map[cacheKey]*cacheItem
	mu       sync.Mutex
	capacity int
}

func newCacheStore(capacity int) cacheStore {
	return cacheStore{
		items:    make(map[cacheKey]*cacheItem),
		capacity: capacity,
	}
}

// getFresh returns the item for the request if it exists and has not expired.
//
// An expired item is removed from the store, so that it doesn't take up the capacity.
func (s *cacheStore) getFresh(key cacheKey, req *dns.Msg, now time.Time) *cacheItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.items[key]
	if item == nil {
		return nil
	}

	if item.ttl(now) <= 0 {
		delete(s.items, key)

		return nil
	}

	if !item.matches(req) {
		return nil
	}

	return item
}

func (s *cacheStore) add(key cacheKey, item *cacheItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[key]; !exists && len(s.items) >= s.capacity {
		for k := range s.items {
			delete(s.items, k)

			break
		}
	}

	s.items[key] = item
}

func (s *cacheStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.items)
}
