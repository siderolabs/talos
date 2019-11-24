// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reg

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	networkapi "github.com/talos-systems/talos/api/network"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
)

// Registrator is the concrete type that implements the factory.Registrator and
// networkapi.NetworkServer interfaces.
type Registrator struct {
	Networkd *networkd.Networkd
	Conn     *rtnetlink.Conn
}

// NewRegistrator builds new Registrator instance.
func NewRegistrator(n *networkd.Networkd) *Registrator {
	nlConn, err := rtnetlink.Dial(nil)
	if err != nil {
		log.Fatal(err)
	}

	return &Registrator{
		Networkd: n,
		Conn:     nlConn,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	networkapi.RegisterNetworkServer(s, r)
}

// Routes returns the hosts routing table.
func (r *Registrator) Routes(ctx context.Context, in *empty.Empty) (reply *networkapi.RoutesReply, err error) {
	list, err := r.Conn.Route.List()
	if err != nil {
		return nil, fmt.Errorf("failed to get route list: %w", err)
	}

	routes := []*networkapi.Route{}

	for _, rMesg := range list {
		ifaceData, err := r.Conn.Link.Get((rMesg.Attributes.OutIface))
		if err != nil {
			log.Printf("failed to get interface details for interface index %d: %v", rMesg.Attributes.OutIface, err)
			// TODO: Remove once we get this sorted on why there's a
			// failure here
			log.Printf("%+v", rMesg)

			continue
		}

		routes = append(routes, &networkapi.Route{
			Interface:   ifaceData.Attributes.Name,
			Destination: toCIDR(rMesg.Family, rMesg.Attributes.Dst, int(rMesg.DstLength)),
			Gateway:     toCIDR(rMesg.Family, rMesg.Attributes.Gateway, 32),
			Metric:      rMesg.Attributes.Priority,
			Scope:       uint32(rMesg.Scope),
			Source:      toCIDR(rMesg.Family, rMesg.Attributes.Src, int(rMesg.SrcLength)),
			Family:      networkapi.AddressFamily(rMesg.Family),
			Protocol:    networkapi.RouteProtocol(rMesg.Protocol),
			Flags:       rMesg.Flags,
		})
	}

	return &networkapi.RoutesReply{
		Response: []*networkapi.RoutesResponse{
			{
				Routes: routes,
			},
		},
	}, nil
}

// Interfaces returns the hosts network interfaces and addresses.
func (r *Registrator) Interfaces(ctx context.Context, in *empty.Empty) (reply *networkapi.InterfacesReply, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return reply, err
	}

	resp := &networkapi.InterfacesResponse{}

	for _, iface := range ifaces {
		ifaceaddrs, err := iface.Addrs()
		if err != nil {
			return reply, err
		}

		addrs := make([]string, 0, len(ifaceaddrs))
		for _, addr := range ifaceaddrs {
			addrs = append(addrs, addr.String())
		}

		ifmsg := &networkapi.Interface{
			Index:        uint32(iface.Index),
			Mtu:          uint32(iface.MTU),
			Name:         iface.Name,
			Hardwareaddr: iface.HardwareAddr.String(),
			Flags:        networkapi.InterfaceFlags(iface.Flags),
			Ipaddress:    addrs,
		}

		resp.Interfaces = append(resp.Interfaces, ifmsg)
	}

	return &networkapi.InterfacesReply{
		Response: []*networkapi.InterfacesResponse{
			resp,
		},
	}, nil
}

func toCIDR(family uint8, prefix net.IP, prefixLen int) string {
	netLen := 32

	if family == unix.AF_INET6 {
		netLen = 128
	}

	// Set a friendly readable value instead of "<nil>"
	if prefix == nil {
		switch family {
		case unix.AF_INET6:
			prefix = net.ParseIP("::")
		case unix.AF_INET:
			prefix = net.ParseIP("0.0.0.0")
		}
	}

	ipNet := &net.IPNet{
		IP:   prefix,
		Mask: net.CIDRMask(prefixLen, netLen),
	}

	return ipNet.String()
}
