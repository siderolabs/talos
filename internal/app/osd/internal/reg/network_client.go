/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg // nolint: dupl

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	networkapi "github.com/talos-systems/talos/api/network"
	"github.com/talos-systems/talos/pkg/constants"
)

// NetworkClient is a gRPC client for init service API
type NetworkClient struct {
	networkapi.NetworkClient
}

// NewNetworkClient initializes new client and connects to networkd
func NewNetworkClient() (*NetworkClient, error) {
	conn, err := grpc.Dial("unix:"+constants.NetworkdSocketPath,
		grpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return &NetworkClient{
		NetworkClient: networkapi.NewNetworkClient(conn),
	}, nil
}

// Routes returns the hosts routing table.
func (c *NetworkClient) Routes(ctx context.Context, in *empty.Empty) (*networkapi.RoutesReply, error) {
	return c.NetworkClient.Routes(ctx, in)
}

// Interfaces returns the hosts network interfaces and addresses.
func (c *NetworkClient) Interfaces(ctx context.Context, in *empty.Empty) (*networkapi.InterfacesReply, error) {
	return c.NetworkClient.Interfaces(ctx, in)
}
