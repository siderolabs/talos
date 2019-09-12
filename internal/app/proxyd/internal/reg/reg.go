/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/proxyd/internal/frontend"
	"github.com/talos-systems/talos/internal/app/proxyd/proto"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.Init interfaces.
type Registrator struct {
	Proxyd *frontend.ReverseProxy
}

// NewRegistrator builds new Registrator instance
func NewRegistrator(proxy *frontend.ReverseProxy) *Registrator {
	return &Registrator{
		Proxyd: proxy,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterProxydServer(s, r)
}

// Backends exposes the internal state of backends in proxyd
func (r *Registrator) Backends(ctx context.Context, in *empty.Empty) (reply *proto.BackendsReply, err error) {
	reply = &proto.BackendsReply{}
	for _, be := range r.Proxyd.Backends() {
		protobe := &proto.Backend{
			Id:          be.UID,
			Addr:        be.Addr,
			Connections: be.Connections,
		}
		reply.Backends = append(reply.Backends, protobe)
	}

	return reply, err
}
