// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"flag"
	"log"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/reg"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
)

var configPath *string

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	configPath = flag.String("config", "", "the path to the config")

	flag.Parse()
}

func main() {
	nwd, err := networkd.New()
	if err != nil {
		log.Fatal(err)
	}

	// Convert links to nic
	log.Println("discovering local network interfaces")

	var netconf networkd.NetConf

	if netconf, err = nwd.Discover(); err != nil {
		log.Fatal(err)
	}

	content, err := config.FromFile(*configPath)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	config, err := config.New(content)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	log.Println("overlaying config network configuration")

	if err = netconf.BuildOptions(config); err != nil {
		log.Fatal(err)
	}

	// Configure specified interface
	netIfaces := make([]*nic.NetworkInterface, 0, len(netconf))

	for link, opts := range netconf {
		var iface *nic.NetworkInterface

		log.Printf("creating interface %s", link.Name)

		iface, err = nic.Create(link, opts...)
		if err != nil {
			log.Fatal(err)
		}

		if iface.IsIgnored() {
			continue
		}

		netIfaces = append(netIfaces, iface)
	}

	// kick off the addressing mechanism
	// Add any necessary routes
	log.Println("configuring interface addressing")

	if err = nwd.Configure(netIfaces...); err != nil {
		log.Fatal(err)
	}

	log.Println("interface configuration")
	nwd.PrintState()

	log.Println("starting renewal watcher")
	// handle dhcp renewal
	go nwd.Renew(netIfaces...)

	log.Fatalf("%+v", factory.ListenAndServe(
		reg.NewRegistrator(nwd),
		factory.Network("unix"),
		factory.SocketPath(constants.NetworkdSocketPath),
	),
	)
}
