// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend

import (
	"context"
	"sync"

	"github.com/siderolabs/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var _ proxy.Backend = (*Local)(nil)

// Local implements local backend (proxying one2one to local service).
type Local struct {
	name       string
	socketPath string

	mu   sync.Mutex
	conn *grpc.ClientConn
}

// NewLocal builds new Local backend.
func NewLocal(name, socketPath string) *Local {
	return &Local{
		name:       name,
		socketPath: socketPath,
	}
}

func (l *Local) String() string {
	return l.name
}

// GetConnection returns a grpc connection to the backend.
func (l *Local) GetConnection(ctx context.Context, _ string) (context.Context, *grpc.ClientConn, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	md = md.Copy()

	authz.SetMetadata(md, authz.GetRoles(ctx))

	outCtx := metadata.NewOutgoingContext(ctx, md)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.conn != nil {
		return outCtx, l.conn, nil
	}

	var err error
	l.conn, err = grpc.NewClient(
		"unix:"+l.socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(constants.GRPCMaxMessageSize),
			grpc.ForceCodecV2(proxy.Codec()),
		),
		grpc.WithSharedWriteBuffer(true),
	)

	return outCtx, l.conn, err
}

// AppendInfo is called to enhance response from the backend with additional data.
func (l *Local) AppendInfo(_ bool, resp []byte) ([]byte, error) {
	return resp, nil
}

// BuildError is called to convert error from upstream into response field.
func (l *Local) BuildError(bool, error) ([]byte, error) {
	return nil, nil
}
