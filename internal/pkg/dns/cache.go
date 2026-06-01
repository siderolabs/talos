// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/cache"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

const requestTimeout = 4500 * time.Millisecond

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

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
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

// Clear clears the cache.
func (c *Cache) Clear() { c.cache.Clear() }

// clientWrite returns true if the response has been written to the client.
func clientWrite(rcode int) bool {
	switch rcode {
	case dns.RcodeServerFailure, dns.RcodeRefused, dns.RcodeFormatError, dns.RcodeNotImplemented:
		return false
	default:
		return true
	}
}
