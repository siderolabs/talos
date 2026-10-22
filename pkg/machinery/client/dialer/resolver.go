// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dialer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
)

const (
	dynamicProxyResolverScheme = "talosdynamicproxy"
	dnsResolverScheme          = "dns"
	defaultPort                = "443"
)

// DynamicProxy pairs a gRPC resolver with a dialer, so that both agree on whether the proxy
// from the environment applies to the connection endpoint.
//
// gRPC has its own support for deferring DNS resolution of the target to the HTTP proxy, but it is
// disabled as soon as a custom dialer is used. With the default `dns` resolver the custom dialer
// receives resolved IP addresses, so the proxy gets `CONNECT <ip>:<port>` which is rejected by the
// proxies filtering by hostname, and domain-based `no_proxy` rules never match.
//
// The resolver evaluates the proxy settings from the environment for the endpoint:
//
//   - if a proxy applies to the endpoint, the endpoint is passed as is to the dialer, so the proxy
//     performs DNS resolution and `CONNECT` carries the hostname;
//   - otherwise, the resolver delegates to the standard gRPC `dns` resolver, and remembers the
//     addresses it produced, so that the dialer connects to them directly even if the proxy settings
//     would match the IP address (e.g. with a domain-based `no_proxy` rule).
//
// The proxy settings are re-evaluated every time gRPC requests re-resolution (e.g. on connection failure),
// so the changes in the environment are picked up without restarting the client.
//
// Use Target to build the gRPC target and DialOptions to configure the client:
//
//	proxy := dialer.NewDynamicProxy(nil)
//	conn, err := grpc.NewClient(proxy.Target("discovery.talos.dev:443"), proxy.DialOptions()...)
//
// A single DynamicProxy might be shared by multiple gRPC clients.
type DynamicProxy struct {
	tlsConfigFunc func() *tls.Config

	mu sync.Mutex
	// locally resolved addresses, per resolver (there's one resolver per gRPC client)
	local map[*dynamicProxyResolver]map[string]struct{}
}

// NewDynamicProxy creates a new DynamicProxy.
//
// The tlsConfigFunc is used to connect to HTTPS proxies, it might be nil.
func NewDynamicProxy(tlsConfigFunc func() *tls.Config) *DynamicProxy {
	if tlsConfigFunc == nil {
		tlsConfigFunc = func() *tls.Config { return &tls.Config{} }
	}

	return &DynamicProxy{
		tlsConfigFunc: tlsConfigFunc,
		local:         map[*dynamicProxyResolver]map[string]struct{}{},
	}
}

// Target wraps the endpoint (`host[:port]`) into a gRPC target handled by the DynamicProxy resolver.
func (p *DynamicProxy) Target(endpoint string) string {
	return dynamicProxyResolverScheme + ":///" + endpoint
}

// DialOptions returns gRPC dial options which should be used with the target built by Target.
func (p *DynamicProxy) DialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithResolvers(dynamicProxyResolverBuilder{proxy: p}),
		grpc.WithContextDialer(p.Dial),
	}
}

// Dial is a gRPC context dialer which connects directly to the addresses resolved locally,
// and otherwise behaves as DynamicProxyDialerWithTLSConfig.
func (p *DynamicProxy) Dial(ctx context.Context, addr string) (net.Conn, error) {
	if p.resolvedLocally(addr) {
		return NetDialerWithTCPKeepalive().DialContext(ctx, "tcp", addr)
	}

	return DynamicProxyDialerWithTLSConfig(p.tlsConfigFunc)(ctx, addr)
}

func (p *DynamicProxy) resolvedLocally(addr string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, addrs := range p.local {
		if _, ok := addrs[addr]; ok {
			return true
		}
	}

	return false
}

func (p *DynamicProxy) setResolvedLocally(r *dynamicProxyResolver, addresses []resolver.Address) {
	addrs := make(map[string]struct{}, len(addresses))

	for _, address := range addresses {
		addrs[address.Addr] = struct{}{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.local[r] = addrs
}

func (p *DynamicProxy) clearResolvedLocally(r *dynamicProxyResolver) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.local, r)
}

type dynamicProxyResolverBuilder struct {
	proxy *DynamicProxy

	// fallback is the resolver builder used when no proxy applies to the endpoint.
	//
	// If nil, the standard gRPC `dns` resolver is used.
	fallback resolver.Builder
}

// Scheme implements resolver.Builder.
func (dynamicProxyResolverBuilder) Scheme() string {
	return dynamicProxyResolverScheme
}

// Build implements resolver.Builder.
func (b dynamicProxyResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	fallback := b.fallback
	if fallback == nil {
		fallback = resolver.Get(dnsResolverScheme)
	}

	if fallback == nil {
		return nil, fmt.Errorf("%s resolver: %q resolver is not registered", dynamicProxyResolverScheme, dnsResolverScheme)
	}

	endpoint, err := ensurePort(target.Endpoint())
	if err != nil {
		return nil, fmt.Errorf("%s resolver: %w", dynamicProxyResolverScheme, err)
	}

	// keep everything but the scheme, so that e.g. `dns://<authority>/` form still works
	fallbackTarget := target
	fallbackTarget.URL.Scheme = fallback.Scheme()

	r := &dynamicProxyResolver{
		proxy:          b.proxy,
		cc:             cc,
		opts:           opts,
		endpoint:       endpoint,
		fallback:       fallback,
		fallbackTarget: fallbackTarget,
	}

	if err = r.reconcileLocked(); err != nil {
		return nil, fmt.Errorf("%s resolver: %w", dynamicProxyResolverScheme, err)
	}

	return r, nil
}

type resolverMode int

const (
	modeUnset resolverMode = iota
	modeProxied
	modeDirect
)

type dynamicProxyResolver struct {
	proxy          *DynamicProxy
	cc             resolver.ClientConn
	opts           resolver.BuildOptions
	fallback       resolver.Builder
	fallbackTarget resolver.Target
	endpoint       string

	mu       sync.Mutex
	mode     resolverMode
	closed   bool
	delegate resolver.Resolver // set only in modeDirect
}

// ResolveNow implements resolver.Resolver.
func (r *dynamicProxyResolver) ResolveNow(opts resolver.ResolveNowOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	if err := r.reconcileLocked(); err != nil {
		r.cc.ReportError(fmt.Errorf("%s resolver: %w", dynamicProxyResolverScheme, err))

		return
	}

	if r.delegate != nil {
		r.delegate.ResolveNow(opts)
	}
}

// Close implements resolver.Resolver.
func (r *dynamicProxyResolver) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true

	r.stopDelegateLocked()
}

// stopDelegateLocked stops the fallback resolver (if any) and forgets the addresses it resolved.
func (r *dynamicProxyResolver) stopDelegateLocked() {
	if r.delegate != nil {
		r.delegate.Close()
		r.delegate = nil
	}

	r.proxy.clearResolvedLocally(r)
}

// reconcileLocked re-evaluates the proxy settings for the endpoint and switches the mode if required.
func (r *dynamicProxyResolver) reconcileLocked() error {
	proxyURL, err := mapAddress(r.endpoint)
	if err != nil {
		return fmt.Errorf("failed to determine proxy for %q: %w", r.endpoint, err)
	}

	proxied := proxyURL != nil

	if (proxied && r.mode == modeProxied) || (!proxied && r.mode == modeDirect) {
		return nil
	}

	// the mode changes, so stop the delegate first, so that it doesn't push stale updates
	r.stopDelegateLocked()

	if proxied {
		r.mode = modeProxied

		// pass the endpoint unresolved, the proxy is going to resolve it
		addresses := []resolver.Address{{Addr: r.endpoint}}

		// the error from UpdateState is ignored: gRPC requests re-resolution itself if the state is rejected
		r.cc.UpdateState(resolver.State{ //nolint:errcheck
			Addresses: addresses,
			Endpoints: []resolver.Endpoint{{Addresses: addresses}},
		})

		return nil
	}

	delegate, err := r.fallback.Build(r.fallbackTarget, &localClientConn{ClientConn: r.cc, r: r}, r.opts)
	if err != nil {
		return fmt.Errorf("failed to build %q resolver: %w", r.fallback.Scheme(), err)
	}

	r.mode = modeDirect
	r.delegate = delegate

	return nil
}

// localClientConn records the addresses produced by the fallback resolver, so that the dialer
// connects to them directly.
type localClientConn struct {
	resolver.ClientConn

	r *dynamicProxyResolver
}

// UpdateState implements resolver.ClientConn.
func (cc *localClientConn) UpdateState(state resolver.State) error {
	cc.r.proxy.setResolvedLocally(cc.r, state.Addresses)

	return cc.ClientConn.UpdateState(state)
}

// NewAddress implements resolver.ClientConn.
func (cc *localClientConn) NewAddress(addresses []resolver.Address) {
	cc.r.proxy.setResolvedLocally(cc.r, addresses)

	cc.ClientConn.NewAddress(addresses) //nolint:staticcheck
}

// ensurePort normalizes the endpoint to the `host:port` form, defaulting the port the same way the gRPC `dns` resolver does.
func ensurePort(endpoint string) (string, error) {
	if endpoint == "" {
		return "", errors.New("missing address")
	}

	if ip := net.ParseIP(endpoint); ip != nil {
		return net.JoinHostPort(endpoint, defaultPort), nil
	}

	if host, port, err := net.SplitHostPort(endpoint); err == nil {
		if port == "" {
			return "", fmt.Errorf("missing port after port-separator colon in %q", endpoint)
		}

		if host == "" {
			host = "localhost"
		}

		return net.JoinHostPort(host, port), nil
	}

	if host, port, err := net.SplitHostPort(endpoint + ":" + defaultPort); err == nil {
		return net.JoinHostPort(host, port), nil
	}

	return "", fmt.Errorf("invalid address %q", endpoint)
}
