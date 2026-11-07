// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package doh implements a DNS-over-HTTPS (RFC 8484) upstream proxy.
//
// The proxy sends the queries as POST requests with the DNS wire format body,
// using an [http.Client] with a custom dialer that redirects direct upstream
// connections to a fixed IP so the request URL host is used purely for
// SNI/certificate verification and the URL path. When an HTTPS proxy is
// configured, the dialer passes the proxy address through so HTTPS_PROXY/NO_PROXY
// are honored.
package doh

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"

	"github.com/siderolabs/talos/internal/pkg/dns/internal/probe"
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

const (
	// mimeType is the media type of DNS messages over HTTP (RFC 8484, section 6).
	mimeType = "application/dns-message"
	// path is the well-known URL path of the DoH endpoint (RFC 8484, section 3).
	path = "/dns-query"
)

// nextProtos is the ALPN list of a DoH client (RFC 8484, section 5.2).
var nextProtos = []string{"h2", "http/1.1"}

// Proxy is a DoH upstream resolver.
//
// It satisfies both the [dns.Upstream] interface used by the resolver handler
// and the [network.Proxy] interface stored in the DNSConn resource.
type Proxy struct {
	// addr is the "host:port" the underlying TCP connection dials. host is the
	// nameserver IP literal (so DoH itself does not require name resolution).
	addr string
	// url is "https://<serverName>/dns-query" — the DoH request URL.
	url string
	// serverName is used as TLS SNI and as the URL host (for certificate
	// validation).
	serverName string

	httpClient *http.Client
	probe      probe.Probe

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
		url:        "https://" + serverName + path,
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
	transport.TLSClientConfig.NextProtos = nextProtos

	p.httpClient = &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}

	return p
}

// Addr returns the upstream address.
func (p *Proxy) Addr() string { return p.addr }

// Fails returns the number of consecutive failures observed since the last
// successful query or health check.
func (p *Proxy) Fails() uint32 { return p.fails.Load() }

// Healthcheck kicks off a round of health checks for this proxy.
//
// The health check sends a ". IN NS" query to the upstream every probe interval
// until it succeeds. If a health check is already in progress, this is a no-op.
func (p *Proxy) Healthcheck() {
	p.probe.Do(func() error {
		ping := new(dns.Msg)
		ping.SetQuestion(".", dns.TypeNS)

		_, err := p.connect(context.Background(), ping)
		if err != nil {
			p.fails.Add(1)

			return err
		}

		p.fails.Store(0)

		return nil
	})
}

// Start sets the interval between health check probes.
func (p *Proxy) Start(duration time.Duration) { p.probe.Start(duration) }

// Close releases any idle connections held by the underlying transport.
func (p *Proxy) Close() {
	if p.closed.Swap(true) {
		return
	}

	p.probe.Stop()

	if t, ok := p.httpClient.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}

// Exchange sends the DNS query as a DoH POST request and returns the parsed
// response. It implements the [dns.Upstream] interface.
//
// The proto parameter (TCP/UDP preference) is irrelevant for DoH — DoH
// always runs over HTTP/2 — and is therefore ignored.
func (p *Proxy) Exchange(ctx context.Context, req *dns.Msg, _ string) (*dns.Msg, error) {
	resp, err := p.connect(ctx, req)
	if err != nil {
		p.fails.Add(1)
	} else {
		p.fails.Store(0)
	}

	return resp, err
}

func (p *Proxy) connect(ctx context.Context, req *dns.Msg) (*dns.Msg, error) {
	if req == nil {
		return nil, errors.New("doh: nil request")
	}

	// the ID is set to zero for HTTP cache friendliness (RFC 8484, section 4.1)
	query := *req
	query.Id = 0

	wire, err := query.Pack()
	if err != nil {
		return nil, fmt.Errorf("doh: pack request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(wire))
	if err != nil {
		return nil, fmt.Errorf("doh: build request: %w", err)
	}

	httpReq.Header.Set("Content-Type", mimeType)
	httpReq.Header.Set("Accept", mimeType)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("doh: %w", err)
	}

	defer httpResp.Body.Close() //nolint:errcheck

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh: upstream returned HTTP %d", httpResp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(httpResp.Body, dns.MaxMsgSize+1))
	if err != nil {
		return nil, fmt.Errorf("doh: read response: %w", err)
	}

	if len(body) > dns.MaxMsgSize {
		return nil, errors.New("doh: response is too large")
	}

	resp := new(dns.Msg)
	if err = resp.Unpack(body); err != nil {
		return nil, fmt.Errorf("doh: parse response: %w", err)
	}

	// restore the original ID
	resp.Id = req.Id

	return resp, nil
}
