/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"fmt"
	"log"

	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/vishvananda/netlink"
)

func setupBonding(netconf userdata.Device) (err error) {
	log.Println("bringing up bonded interface")
	log.Println("**** note **** bonding support is considered experimental")
	bond := netlink.NewLinkBond(netlink.LinkAttrs{Name: netconf.Interface})

	if _, ok := netlink.StringToBondModeMap[netconf.Bond.Mode]; !ok {
		return fmt.Errorf("invalid bond mode for %s", netconf.Interface)
	}
	bond.Mode = netlink.StringToBondModeMap[netconf.Bond.Mode]

	// TODO need a better way to expose bonding configurations
	if _, ok := netlink.StringToBondXmitHashPolicyMap[netconf.Bond.HashPolicy]; !ok {
		return fmt.Errorf("invalid lacp rate for %s", netconf.Interface)
	}
	bond.XmitHashPolicy = netlink.StringToBondXmitHashPolicyMap[netconf.Bond.HashPolicy]

	if _, ok := netlink.StringToBondLacpRateMap[netconf.Bond.LACPRate]; !ok {
		return fmt.Errorf("invalid lacp rate for %s", netconf.Interface)
	}
	bond.LacpRate = netlink.StringToBondLacpRateMap[netconf.Bond.LACPRate]

	// Set up bonding if defined
	var subLink netlink.Link
	for _, subInterface := range netconf.Bond.Interfaces {
		log.Printf("enslaving %s for %s\n", subInterface, netconf.Interface)
		subLink, err = netlink.LinkByName(subInterface)
		if err != nil {
			return err
		}

		// Bring down the interface
		if err = ifdown(subInterface); err != nil {
			log.Printf("failed to set link state to down for %s: %+v", subInterface, err)
			return err
		}

		// Deconfigure all IPs on the sub interface
		var addrs []netlink.Addr
		addrs, err = netlink.AddrList(subLink, netlink.FAMILY_ALL)
		if err != nil {
			log.Printf("failed to get addresses for %s: %+v", subInterface, err)
			return err
		}
		for _, addr := range addrs {
			if err = netlink.AddrDel(subLink, &addr); err != nil {
				log.Printf("failed to delete address for %s %+v: %+v", subInterface, addr, err)
				return err
			}
		}

		// Add sub interface to bond
		if err = netlink.LinkSetBondSlave(subLink, &netlink.Bond{LinkAttrs: *bond.Attrs()}); err != nil {
			return err
		}

		if err = ifup(subInterface); err != nil {
			return err
		}
	}

	return ifup(netconf.Interface)
}
