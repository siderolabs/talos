// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/talos-systems/talos/pkg/grpc/dialer"
)

// Client is a lightweight implementation of CRI client.
type Client struct {
	conn          *grpc.ClientConn
	runtimeClient runtimeapi.RuntimeServiceClient
	imagesClient  runtimeapi.ImageServiceClient
}

// maxMsgSize use 16MB as the default message size limit.
// grpc library default is 4MB.
const maxMsgSize = 1024 * 1024 * 16

// NewClient builds CRI client.
func NewClient(endpoint string, connectionTimeout time.Duration) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(false),
		grpc.WithBackoffMaxDelay(3*time.Second), //nolint:staticcheck
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)),
		grpc.WithContextDialer(dialer.DialUnix()),
	)
	if err != nil {
		return nil, fmt.Errorf("error connecting to CRI: %w", err)
	}

	return &Client{
		conn:          conn,
		runtimeClient: runtimeapi.NewRuntimeServiceClient(conn),
		imagesClient:  runtimeapi.NewImageServiceClient(conn),
	}, nil
}

// Close connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
