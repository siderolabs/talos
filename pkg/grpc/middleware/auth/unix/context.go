// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package unix provides gRPC transport credentials for Unix socket connections
// that authenticate the peer process via SO_PEERCRED (Linux only).
package unix

import (
	"context"

	"google.golang.org/grpc/peer"
)

// PeerCredentials holds Unix socket peer credentials extracted via SO_PEERCRED.
type PeerCredentials struct {
	PID int32
	UID uint32
	GID uint32
}

// AuthType implements credentials.AuthInfo.
func (PeerCredentials) AuthType() string {
	return "unix-peer-creds"
}

// GetPeerCredentials returns the Unix socket peer credentials from the gRPC context.
// Returns false if the context does not contain peer credentials (e.g., non-Unix connections).
func GetPeerCredentials(ctx context.Context) (PeerCredentials, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return PeerCredentials{}, false
	}

	creds, ok := p.AuthInfo.(PeerCredentials)

	return creds, ok
}
