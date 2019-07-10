/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"fmt"
	"log"
	"net"
	"syscall"

	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/vishvananda/netlink"
)

// StaticAddress handles the setting of a static IP address on a
// network interface
func StaticAddress(netconf userdata.Device) (err error) {
	var addr *netlink.Addr
	if addr, err = netlink.ParseAddr(netconf.CIDR); err != nil {
		log.Printf("failed to parse address for interface %s: %+v", netconf.Interface, err)
		return err
	}
	var link netlink.Link
	if link, err = netlink.LinkByName(netconf.Interface); err != nil {
		log.Printf("failed to get interface %s: %+v", netconf.Interface, err)
		return err
	}
	if err = netlink.AddrAdd(link, addr); err != nil && err != syscall.EEXIST {
		log.Printf("failed to add %s to %s: %+v", addr, netconf.Interface, err)
		return err
	}

	// add a gateway route
	var network *net.IPNet
	for _, route := range netconf.Routes {
		_, network, err = net.ParseCIDR(route.Network)
		if err != nil {
			log.Printf("failed to parse static route network %s: %+v", route.Network, err)
			return err
		}

		gw := net.ParseIP(route.Gateway)
		if gw == nil {
			return fmt.Errorf("failed to parse static route gateway %s", route.Gateway)
		}

		route := netlink.Route{LinkIndex: link.Attrs().Index, Dst: network, Gw: gw}
		if err = netlink.RouteAdd(&route); err != nil {
			log.Printf("failed to add route %+v for interface %s", route, netconf.Interface)
			return err
		}
	}

	return err
}
