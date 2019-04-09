/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"log"
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

	return err
}
