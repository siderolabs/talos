/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"regexp"

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

const (
	interfaceResourceRegex = "interfaces?"
	routeResourceRegex     = "routes?"
)

var (
	interfaceResource *regexp.Regexp
	routeResource     *regexp.Regexp
)

// NewRegistrator builds new Registrator instance
func NewRegistrator(data *userdata.UserData) *Registrator {
	interfaceResource = regexp.MustCompile(interfaceResourceRegex)
	routeResource = regexp.MustCompile(routeResourceRegex)

	return &Registrator{
		Data: data,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterNetworkdServer(s, r)
}

// Get acts as the router for Get requests
func (r *Registrator) Get(ctx context.Context, s *proto.GetRequest) (reply *proto.GetReply, err error) {

	switch {
	case interfaceResource.MatchString(s.Resource):
		return getInterfaces(ctx, s), err
	case routeResource.MatchString(s.Resource):
	default:
		return reply, err
	}

	return reply, err
}

func getInterfaces(ctx context.Context, s *proto.GetRequest) (reply *proto.GetReply) {
	nwd := networkd.Instance()
	reply = &proto.GetReply{}

	// TODO create some sort of argparse thing to do this mo betta
	if len(s.Args) == 2 {
		if s.Args[0] == "-i" {
			p := &proto.NetworkInterface{
				Name: nwd.Get(s.Args[1]),
			}
			reply.Interfaces = append(reply.Interfaces, p)
			return reply
		}
	}

	for _, netif := range nwd.List() {
		p := &proto.NetworkInterface{
			Name: netif,
		}
		reply.Interfaces = append(reply.Interfaces, p)
	}

	return reply
}
