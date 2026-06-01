// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package doh implements a DNS-over-HTTPS (RFC 8484) upstream proxy.
//
// The proxy uses [dnshttp] from the codeberg.org/miekg/dns library to build
// DoH requests and parse responses, and an [http.Client] with a custom dialer
// that redirects direct upstream connections to a fixed IP so the request URL
// host is used purely for SNI/certificate verification and the URL path. When
// an HTTPS proxy is configured, the dialer passes the proxy address through so
// HTTPS_PROXY/NO_PROXY are honored. Messages cross the boundary between
// coredns' miekg/dns and codeberg's fork via the DNS wire format.
package doh

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	cdns "codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnshttp"
	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"

	"github.com/siderolabs/talos/pkg/httpdefaults"
)

// dialTimeout caps the time spent establishing the underlying TCP+TLS
// connection to an upstream DoH server.
//
// These timeouts are set lower than a timeout for the entire HostDNS
// request cycle (requestTimeout constants) in internal/pkg/dns.
const dialTimeout = 4 * time.Second

// requestTimeout caps an individual DoH HTTP request.
const requestTimeout = 4 * time.Second

// Proxy is a DoH upstream resolver.
//
// It satisfies both the [dns.Upstream] interface used by the resolver handler
// and the [network.Proxy] interface stored in the DNSConn resource.
type Proxy struct {
	// addr is the "host:port" the underlying TCP connection dials. host is the
	// nameserver IP literal (so DoH itself does not require name resolution).
	addr string
	// url is "https://<serverName>" — the DoH request base URL. The "/dns-query"
	// path is appended by [dnshttp.NewRequest].
	url string
	// serverName is used as TLS SNI and as the URL host (for certificate
	// validation).
	serverName string

	httpClient *http.Client

	fails  atomic.Uint32
	closed atomic.Bool
}

// NewProxy creates a new DoH proxy.
//
// addr is the "ip:port" of the upstream DoH server (typically "ip:443"); the
// underlying transport always dials this address. serverName is used both as
// the TLS SNI/cert verification name and the URL host portion of the DoH
// request (e.g. "https://serverName/dns-query").
func NewProxy(addr, serverName string) *Proxy {
	p := &Proxy{
		addr:       addr,
		serverName: serverName,
		url:        "https://" + serverName,
	}

	transport := &http.Transport{
		// Redirect direct dials to the upstream to the pre-resolved IP so DoH
		// itself does not require recursive DNS. When http.Transport.Proxy
		// returns a URL, address is the proxy's host:port — pass it through so
		// HTTPS_PROXY is honored.
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := &net.Dialer{Timeout: dialTimeout}

			if host, _, err := net.SplitHostPort(address); err == nil && host == serverName {
				return d.DialContext(ctx, network, addr)
			}

			return d.DialContext(ctx, network, address)
		},
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          10,
		TLSHandshakeTimeout:   dialTimeout,
		ExpectContinueTimeout: time.Second,
	}

	// Pick up Talos-managed root CAs (refreshed live via machine config) and
	// honor HTTPS_PROXY/NO_PROXY changes for outbound DoH connections.
	httpdefaults.PatchTransport(transport)

	// PatchTransport replaces TLSClientConfig wholesale, so layer the DoH-specific
	// fields on top while keeping the patched RootCAs.
	transport.TLSClientConfig.ServerName = serverName
	transport.TLSClientConfig.MinVersion = tls.VersionTLS13
	transport.TLSClientConfig.NextProtos = dnshttp.NextProtos

	p.httpClient = &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}

	return p
}

// Addr returns the upstream address.
func (p *Proxy) Addr() string { return p.addr }

// Fails returns the number of consecutive failures observed since the last
// successful query.
func (p *Proxy) Fails() uint32 { return p.fails.Load() }

// Healthcheck is a no-op for DoH: HTTP keep-alive plus the request/response
// cycle itself is sufficient signal, and there is no separate ping primitive
// for DoH.
func (p *Proxy) Healthcheck() {}

// Start is a no-op for DoH (there is no background health-check loop to
// schedule).
func (p *Proxy) Start(time.Duration) {}

// Close releases any idle connections held by the underlying transport.
func (p *Proxy) Close() {
	if p.closed.Swap(true) {
		return
	}

	if t, ok := p.httpClient.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}

// Connect sends the DNS query as a DoH POST request and returns the parsed
// response. It implements the [dns.Upstream] interface.
//
// The opts parameter (TCP/UDP preference, etc.) is irrelevant for DoH — DoH
// always runs over HTTP/2 — and is therefore ignored.
func (p *Proxy) Connect(ctx context.Context, state request.Request, _ proxy.Options) (*dns.Msg, error) {
	resp, err := p.connect(ctx, state)
	if err != nil {
		p.fails.Add(1)
	} else {
		p.fails.Store(0)
	}

	return resp, err
}

func (p *Proxy) connect(ctx context.Context, state request.Request) (*dns.Msg, error) {
	if state.Req == nil {
		return nil, errors.New("doh: nil request")
	}

	cmsg, err := convertToCodeberg(state.Req)
	if err != nil {
		return nil, err
	}

	httpReq, err := dnshttp.NewRequest(http.MethodPost, p.url, cmsg)
	if err != nil {
		return nil, fmt.Errorf("doh: build request: %w", err)
	}

	httpReq = httpReq.WithContext(ctx)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("doh: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close() //nolint:errcheck

		return nil, fmt.Errorf("doh: upstream returned HTTP %d", httpResp.StatusCode)
	}

	cresp, err := dnshttp.Response(httpResp)
	if err != nil {
		return nil, fmt.Errorf("doh: parse response: %w", err)
	}

	resp, err := convertFromCodeberg(cresp)
	if err != nil {
		return nil, err
	}

	// Restore the original ID — dnshttp.NewRequest forces it to zero per RFC 8484.
	resp.Id = state.Req.Id

	return resp, nil
}

// convertToCodeberg crosses the github.com/miekg/dns ↔ codeberg.org/miekg/dns
// type boundary by going through the DNS wire format. Both packages are
// wire-compatible by RFC 1035, so the round-trip preserves the message.
func convertToCodeberg(msg *dns.Msg) (*cdns.Msg, error) {
	wire, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("doh: pack request: %w", err)
	}

	out := &cdns.Msg{Data: wire}
	if err := out.Unpack(); err != nil {
		return nil, fmt.Errorf("doh: convert request: %w", err)
	}

	return out, nil
}

// convertFromCodeberg is the inverse of [convertToCodeberg].
func convertFromCodeberg(msg *cdns.Msg) (*dns.Msg, error) {
	if err := msg.Pack(); err != nil {
		return nil, fmt.Errorf("doh: pack response: %w", err)
	}

	out := new(dns.Msg)
	if err := out.Unpack(msg.Data); err != nil {
		return nil, fmt.Errorf("doh: convert response: %w", err)
	}

	return out, nil
}
