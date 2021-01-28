package route

import (
	"fmt"
	"net"

	"github.com/jsimonetti/rtnetlink"
	"github.com/jsimonetti/rtnetlink/rtnl"
	tnet "github.com/talos-systems/net"
	"golang.org/x/sys/unix"
)

// Route is a representation of a network route.
type Route struct {
	// Destination is the destination network this route provides.
	Destination *net.IPNet

	// Gateway is the router through which the destination may be reached.
	// This option is exclusive of Interface
	Gateway net.IP

	// Interface indicates the route is an interface route, and traffic destinted for the Gateway should be sent through the given network interface.
	// This option is exclusive of Gateway.
	Interface string

	// Metric indicates the "distance" to the destination through this route.
	// This is an integer which allows the control of priority in the case of multiple routes to the same destination.
	Metric uint32
}

// Equal indicates whether two Routes are equal.
func (r *Route) Equal(other *Route) bool {
	if r == nil {
		if other == nil {
			return true
		}

		return false
	}

	if other == nil {
		return false
	}

	if ! r.Destination.IP.Equal(other.Destination.IP) {
		return false
	}

	if ! r.Gateway.Equal(other.Gateway) {
		return false
	}

	if r.Interface != other.Interface {
		return false
	}

	if r.Metric != other.Metric {
		return false
	}

	return true
}

// RTNetlink converts the Route into an rtnetlink Route Message.
func (r *Route) RTNetlink() (rm *rtnetlink.RouteMessage, err error) {
	if r == nil {
		return nil, fmt.Errorf("nil route")
	}

	family := uint8(unix.AF_INET)
	if tnet.IsIPv6(r.Destination.IP) {
		family = unix.AF_INET6
	}
	
	dstOnes, _ := r.Destination.Mask.Size()

	gw := r.Gateway
	if r.Gateway == nil || net.IPv4zero.Equal(r.Gateway) || net.IPv6zero.Equal(r.Gateway) {
		gw = nil
	}

	scope := uint8(unix.RT_SCOPE_UNIVERSE)
	if r.Destination.IP.IsLinkLocalUnicast() || r.Destination.IP.IsLinkLocalMulticast() || r.Destination.IP.To16().IsInterfaceLocalMulticast() {
		scope = unix.RT_SCOPE_LINK
	} else if r.Destination.IP.IsLoopback() {
		scope = unix.RT_SCOPE_HOST
	}

	routeType := uint8(unix.RTN_UNICAST)
	if r.Destination.IP.IsMulticast() {
		routeType = unix.RTN_MULTICAST
	}

	ifaceIndex := 0
	if r.Interface != "" {
		iface, err := net.InterfaceByName(r.Interface)
		if err != nil {
			return nil, fmt.Errorf("failed to determine interface from name %q: %w", r.Interface, err)
		}

		ifaceIndex = iface.Index
	}

	rm = &rtnetlink.RouteMessage{
		Family: family,
		DstLength: uint8(dstOnes),
		SrcLength: 0,
		Tos: 0,
		Table: uint8(unix.RT_TABLE_MAIN),
		Protocol: unix.RTPROT_STATIC,
		Scope: scope,
		Type: routeType,
		Flags: 0,
		Attributes: rtnetlink.RouteAttributes{
			Dst: r.Destination.IP,
			Src: nil,
			Gateway: gw,
			OutIface: uint32(ifaceIndex),
			Priority: r.Metric,
			Table: 0, // override here if we have a specified table
			Mark: 0,
			Expires: nil,
			Metrics: nil,
			Multipath: nil,
		},
	}

	return rm, nil
	
}

// FromRTNL returns a networkd route from the provided rtnl.Route.
func FromRTNL(in *rtnl.Route) (*Route, error) {

}

// FromRTNetlink returns a networkd route from the provided rtnetlink.RouteMessage.
func FromRTNetlink(in *rtnetlink.RouteMessage) (*Route, error) {
	
}

