/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"io"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

// MachineClient is a gRPC client for init service API
type MachineClient struct {
	machineapi.MachineClient
}

// NewMachineClient initializes new client and connects to init
func NewMachineClient() (*MachineClient, error) {
	conn, err := grpc.Dial("unix:"+constants.InitSocketPath,
		grpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return &MachineClient{
		MachineClient: machineapi.NewMachineClient(conn),
	}, nil
}

// Reboot executes init Reboot() API
func (c *MachineClient) Reboot(ctx context.Context, in *empty.Empty) (*machineapi.RebootReply, error) {
	return c.MachineClient.Reboot(ctx, in)
}

// Shutdown executes init Shutdown() API.
func (c *MachineClient) Shutdown(ctx context.Context, in *empty.Empty) (*machineapi.ShutdownReply, error) {
	return c.MachineClient.Shutdown(ctx, in)
}

// Upgrade executes the init Upgrade() API.
func (c *MachineClient) Upgrade(ctx context.Context, in *machineapi.UpgradeRequest) (data *machineapi.UpgradeReply, err error) {
	return c.MachineClient.Upgrade(ctx, in)
}

// Reset executes the init Reset() API.
func (c *MachineClient) Reset(ctx context.Context, in *empty.Empty) (data *machineapi.ResetReply, err error) {
	return c.MachineClient.Reset(ctx, in)
}

// ServiceStart executes the init ServiceStart() API.
func (c *MachineClient) ServiceStart(ctx context.Context, in *machineapi.ServiceStartRequest) (data *machineapi.ServiceStartReply, err error) {
	return c.MachineClient.ServiceStart(ctx, in)
}

// ServiceStop executes the init ServiceStop() API.
func (c *MachineClient) ServiceStop(ctx context.Context, in *machineapi.ServiceStopRequest) (data *machineapi.ServiceStopReply, err error) {
	return c.MachineClient.ServiceStop(ctx, in)
}

// ServiceRestart executes the init ServiceRestart() API.
func (c *MachineClient) ServiceRestart(ctx context.Context, in *machineapi.ServiceRestartRequest) (data *machineapi.ServiceRestartReply, err error) {
	return c.MachineClient.ServiceRestart(ctx, in)
}

// Start executes the init Start() API (deprecated).
//nolint: staticcheck
func (c *MachineClient) Start(ctx context.Context, in *machineapi.StartRequest) (data *machineapi.StartReply, err error) {
	return c.MachineClient.Start(ctx, in)
}

// Stop executes the init Stop() API (deprecated).
//nolint: staticcheck
func (c *MachineClient) Stop(ctx context.Context, in *machineapi.StopRequest) (data *machineapi.StopReply, err error) {
	return c.MachineClient.Stop(ctx, in)
}

// ServiceList executes the init ServiceList() API.
func (c *MachineClient) ServiceList(ctx context.Context, in *empty.Empty) (data *machineapi.ServiceListReply, err error) {
	return c.MachineClient.ServiceList(ctx, in)
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
func (c *MachineClient) CopyOut(req *machineapi.CopyOutRequest, srv machineapi.Machine_CopyOutServer) error {
	client, err := c.MachineClient.CopyOut(srv.Context(), req)
	if err != nil {
		return err
	}

	var msg machineapi.StreamingData

	return copyClientServer(&msg, client, srv)
}

// LS executes the init LS() API.
func (c *MachineClient) LS(req *machineapi.LSRequest, srv machineapi.Machine_LSServer) error {
	client, err := c.MachineClient.LS(srv.Context(), req)
	if err != nil {
		return err
	}

	var msg machineapi.FileInfo

	return copyClientServer(&msg, client, srv)
}

// Mounts implements the machineapi.OSDServer interface.
func (c *MachineClient) Mounts(ctx context.Context, in *empty.Empty) (reply *machineapi.MountsReply, err error) {
	return c.MachineClient.Mounts(ctx, in)
}
