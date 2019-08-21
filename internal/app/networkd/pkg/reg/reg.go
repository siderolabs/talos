/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"log"
	"net"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/app/networkd/proto"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.Init interfaces.
type Registrator struct {
	Networkd *networkd.Networkd
}

// NewRegistrator builds new Registrator instance.
func NewRegistrator(n *networkd.Networkd) *Registrator {
	return &Registrator{
		Networkd: n,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterNetworkdServer(s, r)
}

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

// Interfaces returns the hosts network interfaces and addresses.
func (r *Registrator) Interfaces(ctx context.Context, in *empty.Empty) (reply *proto.InterfacesReply, err error) {
	var (
		ifaces  []*net.Interface
		addrs   []string
		ifaddrs []*net.IPNet
	)

	// List out all interfaces/links
	ifaces, err = r.Networkd.Conn.Links()
	if err != nil {
		return reply, err
	}

	reply = &proto.InterfacesReply{}

	for _, iface := range ifaces {
		addrs = []string{}
		// Gather addresses configured on the given interface
		// both ipv4 and ipv6
		for _, fam := range []int{unix.AF_INET, unix.AF_INET6} {
			ifaddrs, err = r.Networkd.Conn.Addrs(iface, fam)
			if err != nil {
				return reply, err
			}

			for _, ifaddr := range ifaddrs {
				addrs = append(addrs, ifaddr.String())
			}
		}

		ifmsg := &proto.Interface{
			Index:        uint32(iface.Index),
			Mtu:          uint32(iface.MTU),
			Name:         iface.Name,
			Hardwareaddr: iface.HardwareAddr.String(),
			Flags:        proto.InterfaceFlags(iface.Flags),
			Ipaddress:    addrs,
		}

		reply.Interfaces = append(reply.Interfaces, ifmsg)
	}

	return reply, nil
}

func toCIDR(family uint8, prefix net.IP, prefixLen int) string {
	var netLen = 32
	if family == unix.AF_INET6 {
		netLen = 128
	}
	ipNet := &net.IPNet{
		IP:   prefix,
		Mask: net.CIDRMask(prefixLen, netLen),
	}
	return ipNet.String()
}
