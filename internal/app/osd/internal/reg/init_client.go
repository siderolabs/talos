/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/talos/internal/app/init/proto"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"google.golang.org/grpc"
)

// InitServiceClient is a gRPC client for init service API
type InitServiceClient struct {
	proto.InitClient
}

// NewInitServiceClient initializes new client and connects to init
func NewInitServiceClient() (*InitServiceClient, error) {
	conn, err := grpc.Dial("unix:"+constants.InitSocketPath,
		grpc.WithInsecure(),
	)

	if err != nil {
		return nil, err
	}

	return &InitServiceClient{
		InitClient: proto.NewInitClient(conn),
	}, nil
}

// Reboot executes init Reboot() API
func (c *InitServiceClient) Reboot(ctx context.Context, empty *empty.Empty) (*proto.RebootReply, error) {
	return c.InitClient.Reboot(ctx, empty)
}

// Shutdown executes init Shutdown() API.
func (c *InitServiceClient) Shutdown(ctx context.Context, empty *empty.Empty) (*proto.ShutdownReply, error) {
	return c.InitClient.Shutdown(ctx, empty)
}

// Upgrade executes the init Upgrade() API.
func (c *InitServiceClient) Upgrade(ctx context.Context, in *proto.UpgradeRequest) (data *proto.UpgradeReply, err error) {
	return c.InitClient.Upgrade(ctx, in)
}
