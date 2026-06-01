// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package doh

import (
	"context"
	"net"
	"net/http"
)

// DialContext invokes the transport's DialContext for tests.
func (p *Proxy) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return p.httpClient.Transport.(*http.Transport).DialContext(ctx, network, address)
}
