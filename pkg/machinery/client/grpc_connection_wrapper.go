// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type grpcConnectionWrapper struct {
	*grpc.ClientConn

	clusterName string
}

func newGRPCConnectionWrapper(clusterName string, conn *grpc.ClientConn) *grpcConnectionWrapper {
	return &grpcConnectionWrapper{
		ClientConn:  conn,
		clusterName: clusterName,
	}
}

// Invoke performs a unary RPC and returns after the response is received
// into reply.
func (c *grpcConnectionWrapper) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return c.ClientConn.Invoke(c.appendMetadata(ctx), method, args, reply, opts...)
}

// NewStream begins a streaming RPC.
func (c *grpcConnectionWrapper) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return c.ClientConn.NewStream(c.appendMetadata(ctx), desc, method, opts...)
}

func (c *grpcConnectionWrapper) appendMetadata(ctx context.Context) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, "runtime", "Talos")

	if c.clusterName != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "context", c.clusterName)
	}

	return ctx
}
