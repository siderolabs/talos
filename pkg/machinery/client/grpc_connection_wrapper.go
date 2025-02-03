// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"runtime"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
)

type grpcConnectionWrapper struct {
	conn *grpc.ClientConn

	clusterName string
}

func newGRPCConnectionWrapper(clusterName string, conn *grpc.ClientConn) *grpcConnectionWrapper {
	res := &grpcConnectionWrapper{
		conn:        conn,
		clusterName: clusterName,
	}

	runtime.SetFinalizer(res, func(c *grpcConnectionWrapper) { c.Close() }) //nolint:errcheck

	return res
}

func (c *grpcConnectionWrapper) Close() error {
	err := c.conn.Close()

	runtime.SetFinalizer(c, nil)
	runtime.KeepAlive(c)

	return err
}

// NewStream begins a streaming RPC.
func (c *grpcConnectionWrapper) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	res, err := c.conn.NewStream(c.appendMetadata(ctx), desc, method, opts...)

	runtime.KeepAlive(c)

	return res, err
}

// Invoke performs a unary RPC and returns after the response is received
// into reply.
func (c *grpcConnectionWrapper) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	err := c.conn.Invoke(c.appendMetadata(ctx), method, args, reply, opts...)

	runtime.KeepAlive(c)

	return err
}

func (c *grpcConnectionWrapper) CanonicalTarget() string {
	res := c.conn.Target()

	runtime.KeepAlive(c)

	return res
}

func (c *grpcConnectionWrapper) Connect() {
	c.conn.Connect()

	runtime.KeepAlive(c)
}

func (c *grpcConnectionWrapper) GetMethodConfig(method string) grpc.MethodConfig { //nolint:staticcheck
	res := c.conn.GetMethodConfig(method)

	runtime.KeepAlive(c)

	return res
}

func (c *grpcConnectionWrapper) GetState() connectivity.State {
	res := c.conn.GetState()

	runtime.KeepAlive(c)

	return res
}

func (c *grpcConnectionWrapper) ResetConnectBackoff() {
	c.conn.ResetConnectBackoff()

	runtime.KeepAlive(c)
}

func (c *grpcConnectionWrapper) Target() string {
	res := c.conn.Target()

	runtime.KeepAlive(c)

	return res
}

func (c *grpcConnectionWrapper) WaitForStateChange(ctx context.Context, sourceState connectivity.State) bool {
	res := c.conn.WaitForStateChange(ctx, sourceState)

	runtime.KeepAlive(c)

	return res
}

func (c *grpcConnectionWrapper) appendMetadata(ctx context.Context) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, "runtime", "Talos")

	if c.clusterName != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "context", c.clusterName)
	}

	return ctx
}
