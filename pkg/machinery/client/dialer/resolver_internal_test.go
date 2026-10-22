// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dialer

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

type fakeClientConn struct {
	states []resolver.State
	errors []error
}

func (cc *fakeClientConn) UpdateState(state resolver.State) error {
	cc.states = append(cc.states, state)

	return nil
}

func (cc *fakeClientConn) ReportError(err error) {
	cc.errors = append(cc.errors, err)
}

func (cc *fakeClientConn) NewAddress(addresses []resolver.Address) {
	cc.UpdateState(resolver.State{Addresses: addresses}) //nolint:errcheck
}

func (cc *fakeClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return &serviceconfig.ParseResult{Err: errors.New("not supported")}
}

func (cc *fakeClientConn) lastAddrs(t *testing.T) []string {
	t.Helper()

	require.NotEmpty(t, cc.states)

	last := cc.states[len(cc.states)-1]

	addrs := make([]string, 0, len(last.Addresses))
	for _, a := range last.Addresses {
		addrs = append(addrs, a.Addr)
	}

	return addrs
}

// fakeFallbackBuilder stands in for the `dns` resolver: it pushes a fixed address and records the lifecycle calls.
type fakeFallbackBuilder struct {
	builds     int
	closes     int
	resolveNow int
	lastTarget resolver.Target
}

func (b *fakeFallbackBuilder) Scheme() string {
	return "fakedns"
}

func (b *fakeFallbackBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	b.builds++
	b.lastTarget = target

	cc.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: "192.0.2.1:443"}}}) //nolint:errcheck

	return &fakeFallbackResolver{b: b}, nil
}

type fakeFallbackResolver struct {
	b *fakeFallbackBuilder
}

func (r *fakeFallbackResolver) ResolveNow(resolver.ResolveNowOptions) {
	r.b.resolveNow++
}

func (r *fakeFallbackResolver) Close() {
	r.b.closes++
}

func buildTarget(endpoint string) resolver.Target {
	return resolver.Target{URL: url.URL{Scheme: dynamicProxyResolverScheme, Path: "/" + endpoint}}
}

func clearProxyEnv(t *testing.T) {
	t.Helper()

	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(name, "")
	}
}

func TestDynamicProxyResolver(t *testing.T) {
	t.Run("proxied", func(t *testing.T) {
		clearProxyEnv(t)
		t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")

		cc := &fakeClientConn{}
		fallback := &fakeFallbackBuilder{}
		proxy := NewDynamicProxy(nil)

		r, err := dynamicProxyResolverBuilder{proxy: proxy, fallback: fallback}.Build(buildTarget("discovery.talos.dev:443"), cc, resolver.BuildOptions{})
		require.NoError(t, err)

		t.Cleanup(r.Close)

		// the hostname is passed through unresolved
		assert.Equal(t, []string{"discovery.talos.dev:443"}, cc.lastAddrs(t))
		assert.Len(t, cc.states[len(cc.states)-1].Endpoints, 1)
		assert.Zero(t, fallback.builds)
		assert.False(t, proxy.resolvedLocally("discovery.talos.dev:443"))

		// re-resolution in proxied mode is a no-op
		r.ResolveNow(resolver.ResolveNowOptions{})

		assert.Len(t, cc.states, 1)
		assert.Zero(t, fallback.builds)
		assert.Empty(t, cc.errors)
	})

	t.Run("proxied default port", func(t *testing.T) {
		clearProxyEnv(t)
		t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")

		cc := &fakeClientConn{}

		r, err := dynamicProxyResolverBuilder{proxy: NewDynamicProxy(nil), fallback: &fakeFallbackBuilder{}}.Build(buildTarget("discovery.talos.dev"), cc, resolver.BuildOptions{})
		require.NoError(t, err)

		t.Cleanup(r.Close)

		assert.Equal(t, []string{"discovery.talos.dev:443"}, cc.lastAddrs(t))
	})

	t.Run("direct", func(t *testing.T) {
		clearProxyEnv(t)

		cc := &fakeClientConn{}
		fallback := &fakeFallbackBuilder{}
		proxy := NewDynamicProxy(nil)

		r, err := dynamicProxyResolverBuilder{proxy: proxy, fallback: fallback}.Build(buildTarget("discovery.talos.dev:443"), cc, resolver.BuildOptions{})
		require.NoError(t, err)

		assert.Equal(t, 1, fallback.builds)
		assert.Equal(t, "fakedns", fallback.lastTarget.URL.Scheme)
		assert.Equal(t, "discovery.talos.dev:443", fallback.lastTarget.Endpoint())
		assert.Equal(t, []string{"192.0.2.1:443"}, cc.lastAddrs(t))

		// the dialer should connect directly to the locally resolved address even if the proxy would match it
		assert.True(t, proxy.resolvedLocally("192.0.2.1:443"))
		assert.False(t, proxy.resolvedLocally("192.0.2.2:443"))

		// re-resolution is forwarded to the delegate
		r.ResolveNow(resolver.ResolveNowOptions{})
		assert.Equal(t, 1, fallback.resolveNow)
		assert.Equal(t, 1, fallback.builds)

		r.Close()
		assert.Equal(t, 1, fallback.closes)
		assert.False(t, proxy.resolvedLocally("192.0.2.1:443"))

		// no calls after close
		r.ResolveNow(resolver.ResolveNowOptions{})
		assert.Equal(t, 1, fallback.resolveNow)
		assert.Empty(t, cc.errors)
	})

	t.Run("no_proxy", func(t *testing.T) {
		clearProxyEnv(t)
		t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")
		t.Setenv("NO_PROXY", ".talos.dev")

		cc := &fakeClientConn{}
		fallback := &fakeFallbackBuilder{}
		proxy := NewDynamicProxy(nil)

		r, err := dynamicProxyResolverBuilder{proxy: proxy, fallback: fallback}.Build(buildTarget("discovery.talos.dev:443"), cc, resolver.BuildOptions{})
		require.NoError(t, err)

		t.Cleanup(r.Close)

		assert.Equal(t, 1, fallback.builds)
		assert.Equal(t, []string{"192.0.2.1:443"}, cc.lastAddrs(t))
		assert.True(t, proxy.resolvedLocally("192.0.2.1:443"))
	})

	t.Run("switch modes", func(t *testing.T) {
		clearProxyEnv(t)

		cc := &fakeClientConn{}
		fallback := &fakeFallbackBuilder{}
		proxy := NewDynamicProxy(nil)

		r, err := dynamicProxyResolverBuilder{proxy: proxy, fallback: fallback}.Build(buildTarget("discovery.talos.dev:443"), cc, resolver.BuildOptions{})
		require.NoError(t, err)

		t.Cleanup(r.Close)

		assert.Equal(t, []string{"192.0.2.1:443"}, cc.lastAddrs(t))

		// proxy appears in the environment (e.g. EnvironmentConfig applied), re-resolution picks it up
		t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")

		r.ResolveNow(resolver.ResolveNowOptions{})

		assert.Equal(t, 1, fallback.closes, "delegate should be closed when switching to proxied mode")
		assert.Equal(t, []string{"discovery.talos.dev:443"}, cc.lastAddrs(t))
		assert.False(t, proxy.resolvedLocally("192.0.2.1:443"))

		// proxy goes away, back to the delegate
		t.Setenv("HTTPS_PROXY", "")

		r.ResolveNow(resolver.ResolveNowOptions{})

		assert.Equal(t, 2, fallback.builds, "delegate should be rebuilt when switching to direct mode")
		assert.Equal(t, []string{"192.0.2.1:443"}, cc.lastAddrs(t))
		assert.True(t, proxy.resolvedLocally("192.0.2.1:443"))
		assert.Empty(t, cc.errors)
	})
}

func TestEnsurePort(t *testing.T) {
	for _, tt := range []struct {
		in       string
		expected string
		fail     bool
	}{
		{in: "discovery.talos.dev:443", expected: "discovery.talos.dev:443"},
		{in: "discovery.talos.dev", expected: "discovery.talos.dev:443"},
		{in: "10.0.0.1", expected: "10.0.0.1:443"},
		{in: "10.0.0.1:8080", expected: "10.0.0.1:8080"},
		{in: "2001:db8::1", expected: "[2001:db8::1]:443"},
		{in: "[2001:db8::1]:8080", expected: "[2001:db8::1]:8080"},
		{in: ":8080", expected: "localhost:8080"},
		{in: "", fail: true},
		{in: "host:", fail: true},
	} {
		t.Run(tt.in, func(t *testing.T) {
			out, err := ensurePort(tt.in)
			if tt.fail {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}
