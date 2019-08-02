/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/userdata"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}

func main() {
	var (
		netconf networkd.NetConf
		ud      *userdata.UserData
	)

	nwd, err := networkd.New()
	if err != nil {
		log.Fatal(err)
	}

	// Is there any value in returning the low level link or
	// should the translation to nic be handled here?
	// links := nwd.Discover()

	// Convert links to nic
	log.Println("Discovering local network interfaces")
	netconf, err = nwd.Discover()
	if err != nil {
		log.Fatal(err)
	}

	// Load up userdata
	ud, err = userdata.Open("/var/userdata.yaml")
	if err != nil {
		log.Printf("failed to read userdata %s, using defaults: %+v", "/var/userdata.yaml", err)
	}

	log.Println("Overlaying userdata network configuration")
	// Update nic with userdata specified options
	if err = netconf.OverlayUserData(ud); err != nil {
		log.Fatal(err)
	}

	// Configure specified interface
	netIfaces := make([]*nic.NetworkInterface, 0, len(netconf))
	for name, opts := range netconf {
		var iface *nic.NetworkInterface
		log.Printf("Creating interface %s", name)
		iface, err = nic.Create(opts...)
		if err != nil {
			log.Fatal(err)
		}

		netIfaces = append(netIfaces, iface)
	}

	// kick off the addressing mechanism
	// Add any necessary routes
	log.Println("Configuring interface addressing")
	if err = nwd.Configure(netIfaces...); err != nil {
		log.Fatal(err)
	}

	// handle dhcp renewal
	nwd.Renew(netIfaces...)
}
