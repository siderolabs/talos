// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package doh

import (
	"context"
	"crypto/x509"
	"net"
	"net/http"
)

// DialContext invokes the transport's DialContext for tests.
func (p *Proxy) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return p.httpClient.Transport.(*http.Transport).DialContext(ctx, network, address)
}

// SetRootCAs sets the root CAs used to verify the upstream certificate for tests.
func (p *Proxy) SetRootCAs(pool *x509.CertPool) {
	p.httpClient.Transport.(*http.Transport).TLSClientConfig.RootCAs = pool
}
