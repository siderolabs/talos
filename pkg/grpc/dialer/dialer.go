// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dialer

import (
	"context"
	"fmt"
	"net"
	"net/url"
)

// DialUnix is used as a parameter for 'grpc.WithContextDialer' to bypass the
// default dialer of gRPC to ensure that proxy vars are not used.
func DialUnix() func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		u, err := url.Parse(addr)
		if err != nil {
			return nil, err
		}

		if u.Scheme != "unix" {
			return nil, fmt.Errorf("invalid scheme: %q", u.Scheme)
		}

		return net.Dial(u.Scheme, u.Path)
	}
}
