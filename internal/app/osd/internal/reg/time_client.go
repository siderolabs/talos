/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg // nolint: dupl

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	timeapi "github.com/talos-systems/talos/api/time"
	"github.com/talos-systems/talos/pkg/constants"
)

// TimeClient is a gRPC client for init service API
type TimeClient struct {
	timeapi.TimeClient
}

// NewTimeClient initializes new client and connects to ntpd
func NewTimeClient() (*TimeClient, error) {
	conn, err := grpc.Dial("unix:"+constants.NtpdSocketPath,
		grpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return &TimeClient{
		TimeClient: timeapi.NewTimeClient(conn),
	}, nil
}

// Time issues a query to the configured ntp server and displays the results
func (c *TimeClient) Time(ctx context.Context, in *empty.Empty) (*timeapi.TimeReply, error) {
	return c.TimeClient.Time(ctx, in)
}

// TimeCheck issues a query to the specified ntp server and displays the results
func (c *TimeClient) TimeCheck(ctx context.Context, in *timeapi.TimeRequest) (*timeapi.TimeReply, error) {
	return c.TimeClient.TimeCheck(ctx, in)
}
