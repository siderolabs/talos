/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg // nolint: dupl

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/talos/internal/app/networkd/proto"
	"github.com/talos-systems/talos/pkg/constants"
	"google.golang.org/grpc"
)

// NetworkdClient is a gRPC client for init service API
type NetworkdClient struct {
	proto.NetworkdClient
}

// NewNetworkdClient initializes new client and connects to networkd
func NewNetworkdClient() (*NetworkdClient, error) {
	conn, err := grpc.Dial("unix:"+constants.NetworkdSocketPath,
		grpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return &NetworkdClient{
		NetworkdClient: proto.NewNetworkdClient(conn),
	}, nil
}

// Routes returns the hosts routing table.
func (c *NetworkdClient) Routes(ctx context.Context, in *empty.Empty) (*proto.RoutesReply, error) {
	return c.NetworkdClient.Routes(ctx, in)
}

// Interfaces returns the hosts network interfaces and addresses.
func (c *NetworkdClient) Interfaces(ctx context.Context, in *empty.Empty) (*proto.InterfacesReply, error) {
	return c.NetworkdClient.Interfaces(ctx, in)
}
