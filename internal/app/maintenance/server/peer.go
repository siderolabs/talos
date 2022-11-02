// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"net"
	"net/netip"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func verifyPeer(ctx context.Context, condition func(netip.Addr) bool) bool {
	remotePeer, ok := peer.FromContext(ctx)
	if !ok {
		return false
	}

	if remotePeer.Addr.Network() != "tcp" {
		return false
	}

	ip, _, err := net.SplitHostPort(remotePeer.Addr.String())
	if err != nil {
		return false
	}

	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	return condition(addr)
}

func assertPeerSideroLink(ctx context.Context) error {
	if !verifyPeer(ctx, func(addr netip.Addr) bool {
		return network.IsULA(addr, network.ULASideroLink)
	}) {
		return status.Error(codes.Unimplemented, "API is not implemented in maintenance mode")
	}

	return nil
}
