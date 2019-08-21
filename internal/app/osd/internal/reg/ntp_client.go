/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/talos/internal/app/ntpd/proto"
	"github.com/talos-systems/talos/pkg/constants"
	"google.golang.org/grpc"
)

// NtpdClient is a gRPC client for init service API
type NtpdClient struct {
	proto.NtpdClient
}

// NewNtpdClient initializes new client and connects to ntpd
func NewNtpdClient() (*NtpdClient, error) {
	conn, err := grpc.Dial("unix:"+constants.NtpdSocketPath,
		grpc.WithInsecure(),
	)

	if err != nil {
		return nil, err
	}

	return &NtpdClient{
		NtpdClient: proto.NewNtpdClient(conn),
	}, nil
}

// Time issues a query to the configured ntp server and displays the results
func (c *NtpdClient) Time(ctx context.Context, in *empty.Empty) (*proto.TimeReply, error) {
	return c.NtpdClient.Time(ctx, in)
}

// TimeCheck issues a query to the specified ntp server and displays the results
func (c *NtpdClient) TimeCheck(ctx context.Context, in *proto.TimeRequest) (*proto.TimeReply, error) {
	return c.NtpdClient.TimeCheck(ctx, in)
}
