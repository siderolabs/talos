// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"net"

	"google.golang.org/grpc/peer"
)

// RemotePeer parses remote peer address from grpc stream context.
func RemotePeer(ctx context.Context) (peerHost string) {
	peerHost = "unknown"

	remote, ok := peer.FromContext(ctx)
	if ok {
		peerHost = AddrFromPeer(remote)
	}

	return
}

// AddrFromPeer extracts peer address from grpc Peer.
func AddrFromPeer(remote *peer.Peer) (peerHost string) {
	peerHost = remote.Addr.String()
	peerHost, _, _ = net.SplitHostPort(peerHost) //nolint:errcheck

	return peerHost
}
