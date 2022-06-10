// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package httpdefaults provides default HTTP client settings for Talos.
package httpdefaults

import (
	"crypto/tls"
	"net/http"
	"net/url"

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
