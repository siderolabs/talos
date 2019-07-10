/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"log"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/app/networkd/proto"
	"github.com/talos-systems/talos/pkg/userdata"
	"google.golang.org/grpc"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.Init interfaces.
type Registrator struct {
	Data *userdata.UserData
}

// NewRegistrator builds new Registrator instance
func NewRegistrator(data *userdata.UserData) *Registrator {
	return &Registrator{
		Data: data,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterNetworkdServer(s, r)
}

func (r *Registrator) Get(ctx context.Context, s *proto.GetRequest) (reply *proto.GetReply, err error) {
	log.Printf("%+v", s)
	nwd, _ := networkd.New()
	nwd.Parse(&userdata.UserData{})
	reply = &proto.GetReply{}

	for _, netif := range nwd.Interfaces {
		p := &proto.NetworkInterface{
			Name: netif.Name,
		}
		reply.Interfaces = append(reply.Interfaces, p)
	}

	return reply, err
}
