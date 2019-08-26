/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"net"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/jsimonetti/rtnetlink"
	"github.com/talos-systems/talos/internal/app/networkd/proto"
	"golang.org/x/sys/unix"
)

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

// InterfaceStats returns the hosts network interfaces and addresses.
func (r *Registrator) InterfaceStats(ctx context.Context, in *proto.InterfaceStatsRequest) (reply *proto.InterfacesReply, err error) {
	ints, err := r.Interfaces(ctx, &empty.Empty{})
	if err != nil {
		return reply, err
	}

	reply = &proto.InterfacesReply{}

	for _, netif := range ints.Interfaces {
		for _, requested := range in.Interfaces {
			if netif.Name == requested {
				reply.Interfaces = append(reply.Interfaces, netif)
			}
		}
	}

	var link rtnetlink.LinkMessage
	for _, netif := range reply.Interfaces {
		link, err = r.Networkd.NlConn.Link.Get(netif.Index)
		if err != nil {
			return reply, err
		}

		netif.Linkstats = &proto.LinkStats{
			RXPackets:         link.Attributes.Stats64.RXPackets,
			TXPackets:         link.Attributes.Stats64.TXPackets,
			RXBytes:           link.Attributes.Stats64.RXBytes,
			TXBytes:           link.Attributes.Stats64.TXBytes,
			RXErrors:          link.Attributes.Stats64.RXErrors,
			TXErrors:          link.Attributes.Stats64.TXErrors,
			RXDropped:         link.Attributes.Stats64.RXDropped,
			TXDropped:         link.Attributes.Stats64.TXDropped,
			Multicast:         link.Attributes.Stats64.Multicast,
			Collisions:        link.Attributes.Stats64.Collisions,
			RXLengthErrors:    link.Attributes.Stats64.RXLengthErrors,
			RXOverErrors:      link.Attributes.Stats64.RXOverErrors,
			RXCRCErrors:       link.Attributes.Stats64.RXCRCErrors,
			RXFrameErrors:     link.Attributes.Stats64.RXFrameErrors,
			RXFIFOErrors:      link.Attributes.Stats64.RXFIFOErrors,
			RXMissedErrors:    link.Attributes.Stats64.RXMissedErrors,
			TXAbortedErrors:   link.Attributes.Stats64.TXAbortedErrors,
			TXCarrierErrors:   link.Attributes.Stats64.TXCarrierErrors,
			TXFIFOErrors:      link.Attributes.Stats64.TXFIFOErrors,
			TXHeartbeatErrors: link.Attributes.Stats64.TXHeartbeatErrors,
			TXWindowErrors:    link.Attributes.Stats64.TXWindowErrors,
			RXCompressed:      link.Attributes.Stats64.RXCompressed,
			TXCompressed:      link.Attributes.Stats64.TXCompressed,
			RXNoHandler:       link.Attributes.Stats64.RXNoHandler,
		}
	}

	return reply, err
}
