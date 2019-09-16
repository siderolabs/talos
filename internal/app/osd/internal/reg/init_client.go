/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"io"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	proto "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/pkg/constants"
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
func (c *InitServiceClient) Reboot(ctx context.Context, in *empty.Empty) (*proto.RebootReply, error) {
	return c.InitClient.Reboot(ctx, in)
}

// Shutdown executes init Shutdown() API.
func (c *InitServiceClient) Shutdown(ctx context.Context, in *empty.Empty) (*proto.ShutdownReply, error) {
	return c.InitClient.Shutdown(ctx, in)
}

// Upgrade executes the init Upgrade() API.
func (c *InitServiceClient) Upgrade(ctx context.Context, in *proto.UpgradeRequest) (data *proto.UpgradeReply, err error) {
	return c.InitClient.Upgrade(ctx, in)
}

// Reset executes the init Reset() API.
func (c *InitServiceClient) Reset(ctx context.Context, in *empty.Empty) (data *proto.ResetReply, err error) {
	return c.InitClient.Reset(ctx, in)
}

// ServiceStart executes the init ServiceStart() API.
func (c *InitServiceClient) ServiceStart(ctx context.Context, in *proto.ServiceStartRequest) (data *proto.ServiceStartReply, err error) {
	return c.InitClient.ServiceStart(ctx, in)
}

// ServiceStop executes the init ServiceStop() API.
func (c *InitServiceClient) ServiceStop(ctx context.Context, in *proto.ServiceStopRequest) (data *proto.ServiceStopReply, err error) {
	return c.InitClient.ServiceStop(ctx, in)
}

// ServiceRestart executes the init ServiceRestart() API.
func (c *InitServiceClient) ServiceRestart(ctx context.Context, in *proto.ServiceRestartRequest) (data *proto.ServiceRestartReply, err error) {
	return c.InitClient.ServiceRestart(ctx, in)
}

// Start executes the init Start() API (deprecated).
//nolint: staticcheck
func (c *InitServiceClient) Start(ctx context.Context, in *proto.StartRequest) (data *proto.StartReply, err error) {
	return c.InitClient.Start(ctx, in)
}

// Stop executes the init Stop() API (deprecated).
//nolint: staticcheck
func (c *InitServiceClient) Stop(ctx context.Context, in *proto.StopRequest) (data *proto.StopReply, err error) {
	return c.InitClient.Stop(ctx, in)
}

// ServiceList executes the init ServiceList() API.
func (c *InitServiceClient) ServiceList(ctx context.Context, in *empty.Empty) (data *proto.ServiceListReply, err error) {
	return c.InitClient.ServiceList(ctx, in)
}

func copyClientServer(msg interface{}, client grpc.ClientStream, srv grpc.ServerStream) error {
	for {
		err := client.RecvMsg(msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		err = srv.SendMsg(msg)
		if err != nil {
			return err
		}
	}

	return nil
}

// CopyOut executes the init CopyOut() API.
func (c *InitServiceClient) CopyOut(req *proto.CopyOutRequest, srv proto.Init_CopyOutServer) error {
	client, err := c.InitClient.CopyOut(srv.Context(), req)
	if err != nil {
		return err
	}

	var msg proto.StreamingData

	return copyClientServer(&msg, client, srv)
}

// LS executes the init LS() API.
func (c *InitServiceClient) LS(req *proto.LSRequest, srv proto.Init_LSServer) error {
	client, err := c.InitClient.LS(srv.Context(), req)
	if err != nil {
		return err
	}

	var msg proto.FileInfo

	return copyClientServer(&msg, client, srv)
}

// DF implements the proto.OSDServer interface.
func (c *InitServiceClient) DF(ctx context.Context, in *empty.Empty) (reply *proto.DFReply, err error) {
	return c.InitClient.DF(ctx, in)
}
