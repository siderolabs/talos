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

	// Set up bonding if defined
	var slaveLink netlink.Link
	for _, bondInterface := range netconf.Bond.Interfaces {
		log.Printf("enslaving %s for %s\n", bondInterface, netconf.Interface)
		slaveLink, err = netlink.LinkByName(bondInterface)
		if err != nil {
			return err
		}

		if err = netlink.LinkSetBondSlave(slaveLink, &netlink.Bond{LinkAttrs: *bond.Attrs()}); err != nil {
			return err
		}

		// TODO do we need to ifup slave interfaces?
	}
	return err
}
