// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package httpdefaults provides default HTTP client settings for Talos.
package httpdefaults

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"

	"golang.org/x/net/http/httpproxy"
)

// PatchTransport updates *http.Transport with Talos-specific settings.
//
// Settings applied here only make sense when running in Talos root filesystem.
func PatchTransport(transport *http.Transport) *http.Transport {
	// Explicitly set the Proxy function to work around proxy.Do
	// once: the environment variables will be reread/initialized each time the
	// http call is made.
	transport.Proxy = func(req *http.Request) (*url.URL, error) {
		return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
	}

	// Override the TLS config to allow refreshing CA list which might be updated
	// via the machine config on the fly.
	transport.TLSClientConfig = &tls.Config{
		RootCAs: RootCAs(),
	}

	return transport
}

// LazyTransport returns a [http.RoundTripper] which builds the underlying transport on first use.
//
// Building a patched transport loads the trusted CA bundle, so this avoids the cost
// for processes which never make an HTTP request through the returned transport.
func LazyTransport(newTransport func() *http.Transport) http.RoundTripper {
	return &lazyTransport{
		newTransport: newTransport,
	}
}

type lazyTransport struct {
	newTransport func() *http.Transport
	transport    atomic.Pointer[http.Transport]
	once         sync.Once
}

func (t *lazyTransport) get() *http.Transport {
	t.once.Do(func() {
		t.transport.Store(t.newTransport())
	})

	return t.transport.Load()
}

// RoundTrip implements [http.RoundTripper].
func (t *lazyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.get().RoundTrip(req)
}

// CloseIdleConnections closes idle connections of the underlying transport, if it was built.
//
// It is called by [http.Client.CloseIdleConnections].
func (t *lazyTransport) CloseIdleConnections() {
	if transport := t.transport.Load(); transport != nil {
		transport.CloseIdleConnections()
	}
}
