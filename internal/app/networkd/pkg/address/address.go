/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package address

import (
	"context"
	"encoding/binary"
	"net"
	"sort"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

// Addressing provides an interface for abstracting the underlying network
// addressing configuration. Currently dhcp(v4) and static methods are
// supported.
type Addressing interface {
	Name() string
	Discover(context.Context, string) error
	Address() net.IP
	Mask() net.IPMask
	MTU() uint32
	TTL() time.Duration
	Family() uint8
	Scope() uint8
	Routes() []*Route
	Resolvers() []net.IP
	Hostname() string
}

// Route is a representation of a network route
type Route = dhcpv4.Route

// AddressMessage generates a rtnetlink.AddressMessage from the underlying
// Addressing implementation. This message will be used to set the network
// interface address.
// nolint: golint
func AddressMessage(method Addressing, idx uint32) *rtnetlink.AddressMessage {
	attrs := rtnetlink.AddressAttributes{
		Address: method.Address(),
	}

	// AF_INET / ipv4 requires some additional configuration ( broadcast addr )
	if method.Family() == unix.AF_INET {
		brd := make(net.IP, len(method.Address()))
		binary.BigEndian.PutUint32(brd, binary.BigEndian.Uint32(method.Address())|^binary.BigEndian.Uint32(method.Mask()))

		attrs.Broadcast = brd
		attrs.Local = method.Address()
	}

	ones, _ := method.Mask().Size()

	// TODO: look at setting dynamic/permanent flag on address
	// TODO: look at setting valid lifetime and preferred lifetime
	// Ref for scope configuration
	// https://elixir.bootlin.com/linux/latest/source/net/ipv4/fib_semantics.c#L919
	// https://unix.stackexchange.com/questions/123084/what-is-the-interface-scope-global-vs-link-used-for
	msg := &rtnetlink.AddressMessage{
		Family:       method.Family(),
		PrefixLength: uint8(ones),
		Scope:        method.Scope(),
		Index:        idx,
		Attributes:   attrs,
	}

	return msg
}

// RouteMessage generates a slice of rtnetlink.RouteMessages from the
// underlying Addressing implementation. These messages will be used to set
// up the routing table.
func RouteMessage(method Addressing, idx uint32) []*rtnetlink.RouteMessage {
	routes := make([]*rtnetlink.RouteMessage, 0, len(method.Routes()))

	var protocol uint8
	switch method.(type) {
	case *DHCP:
		protocol = unix.RTPROT_DHCP
	case *Static:
		protocol = unix.RTPROT_STATIC
	}

	for _, route := range method.Routes() {
		attr := rtnetlink.RouteAttributes{
			OutIface: idx,
			Table:    unix.RT_TABLE_MAIN,
		}

		// Default to scope_link
		var scope uint8
		switch {
		case route.Dest == nil:
			// If no dest set, assume gateway
			scope = unix.RT_SCOPE_UNIVERSE
		case route.Dest.IP.Equal(net.IPv4zero):
			scope = unix.RT_SCOPE_UNIVERSE
		default:
			scope = unix.RT_SCOPE_LINK
		}

		routeMsg := &rtnetlink.RouteMessage{
			Family:   method.Family(),
			Table:    unix.RT_TABLE_MAIN,
			Protocol: protocol,
			Scope:    scope,
			Type:     unix.RTN_UNICAST,
		}

		if route.Dest != nil {
			attr.Dst = route.Dest.IP
			dstLength, _ := route.Dest.Mask.Size()
			routeMsg.DstLength = uint8(dstLength)
		}

		if route.Router != nil {
			attr.Gateway = route.Router
		}

		// TODO figure out if Src is actually needed
		/*
			if r.Src != nil {
				attr.Src = r.Src.IP
				ones, _ := r.Src.Net.Mask.Size()
				routeMsg.SrcLength = uint8(ones)
			}
		*/

		routeMsg.Attributes = attr

		routes = append(routes, routeMsg)
	}

	// Return a sorted list of routes by scope
	// This should allow us to set up link level routes
	// before universal routes. This should help prevent
	// any network unreachable errors
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Scope > routes[j].Scope
	})

	return routes
}
