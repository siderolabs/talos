/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"log"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/networkd/proto"
)

// Routes returns the hosts routing table.
func (r *Registrator) Routes(ctx context.Context, in *empty.Empty) (reply *proto.RoutesReply, err error) {
	list, err := r.Networkd.NlConn.Route.List()
	if err != nil {
		return nil, errors.Errorf("failed to get route list: %v", err)
	}

	routes := []*proto.Route{}

	for _, rMesg := range list {

		ifaceData, err := r.Networkd.Conn.LinkByIndex(int(rMesg.Attributes.OutIface))
		if err != nil {
			log.Printf("failed to get interface details for interface index %d: %v", rMesg.Attributes.OutIface, err)
			// TODO: Remove once we get this sorted on why there's a
			// failure here
			log.Printf("%+v", rMesg)
			continue
		}

		routes = append(routes, &proto.Route{
			Interface:   ifaceData.Name,
			Destination: toCIDR(rMesg.Family, rMesg.Attributes.Dst, int(rMesg.DstLength)),
			Gateway:     rMesg.Attributes.Gateway.String(),
			Metric:      rMesg.Attributes.Priority,
			Scope:       uint32(rMesg.Scope),
			Source:      toCIDR(rMesg.Family, rMesg.Attributes.Src, int(rMesg.SrcLength)),
			Family:      proto.AddressFamily(rMesg.Family),
			Protocol:    proto.RouteProtocol(rMesg.Protocol),
			Flags:       rMesg.Flags,
		})

	}
	return &proto.RoutesReply{
		Routes: routes,
	}, nil
}
