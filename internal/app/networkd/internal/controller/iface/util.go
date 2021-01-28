package iface

import (
	"errors"
	"fmt"
	"net"

	"github.com/jsimonetti/rtnetlink"
	"github.com/jsimonetti/rtnetlink/rtnl"
	tnet "github.com/talos-systems/net"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/route"
	"golang.org/x/sys/unix"
)

// ErrNotFound indicates that the interface was not found by the given parameters.
var ErrNotFound = errors.New("not found")

func interfaceByMAC(mac string) (*net.Interface, error) {
	list, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get list of interfaces: %w", err)
	}

	for _, f := range list {
		if f.HardwareAddr.String() == mac {
			return &f, nil
		}
	}

	return nil, ErrNotFound
}

// this should not be necessary after PR is merged upstream.
// https://github.com/jsimonetti/rtnetlink/pull/100
func getRoutes(c *rtnl.Conn, dst net.IP) (ret []*route.Route, err error) {
	tx := &rtnetlink.RouteMessage{
		Family:     addrFamily(dst),
		Table:      unix.RT_TABLE_MAIN,
		Attributes: rtnetlink.RouteAttributes{
			Dst: dst,
		},
	}

	rx, err := c.Conn.Route.Get(tx)
	if err != nil {
		return nil, err
	}

	for _, rt := range rx {
		ifindex := int(rt.Attributes.OutIface)

		iface, err := c.LinkByIndex(ifindex)
		if err != nil {
			return nil, fmt.Errorf("failed to get link by interface index: %w", err)
		}

		_, dstNet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", rt.Attributes.Dst.String(), rt.DstLength))
		if err != nil {
			return nil, fmt.Errorf("failed to construct CIDR from route destination address and length: %w", err)
		}

		ret = append(ret, &route.Route{
			Destination: dstNet,
			Gateway:     rt.Attributes.Gateway,
			Interface:   iface.Name,
			Metric:      rt.Attributes.Priority,
		})
	}

	return ret, nil
}

func routeExists(c *rtnl.Conn, targetRoute *route.Route) (bool, error) {
	existingRoutes, err := getRoutes(c, targetRoute.Destination.IP)
	if err != nil {
		return false, fmt.Errorf("failed to get existing routes: %w", err)
	}

	for _, r := range existingRoutes {
		if r.Equal(targetRoute) {
			return true, nil
		}
	}

	return false, nil
}

func addrFamily(a net.IP) uint8 {
	if tnet.IsIPv6(a) {
		return unix.AF_INET6
	}
	return unix.AF_INET
}
