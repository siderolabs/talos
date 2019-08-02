/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/jsimonetti/rtnetlink"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
)

// Set up default nameservers
const (
	DefaultPrimaryResolver   = "1.1.1.1"
	DefaultSecondaryResolver = "8.8.8.8"
)

// Networkd provides the high level interaction to configure network interfaces
// on a host system. This currently support addressing configuration via dhcp
// and/or a specified configuration file.
type Networkd struct {
	Conn *rtnetlink.Conn
}

// New instantiates a new rtnetlink connection that is used for all subsequent
// actions
func New() (*Networkd, error) {
	// Handle netlink connection
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return nil, err
	}

	return &Networkd{Conn: conn}, err
}

// Discover enumerates a list of network links on the host and creates a
// base set of interface configuration options
func (n *Networkd) Discover() (NetConf, error) {
	links, err := n.Conn.Link.List()
	if err != nil {
		return NetConf{}, err
	}

	linkmap := NetConf{}

	for _, link := range filterInterfaceByName(links) {
		linkmap[link.Attributes.Name] = parseLinkMessage(link)
	}

	return linkmap, nil
}

// Configure handles the interface configuration portion. This is inclusive of
// the address discovery ( static vs dhcp ) as well as the netlink interaction
// to set an address on the link and create any routes.
func (n *Networkd) Configure(ifaces ...*nic.NetworkInterface) error {
	var (
		err       error
		resolvers []net.IP
	)

	for _, iface := range ifaces {
		log.Printf("configuring %+v\n", iface)
		// Attempt dhcp against all unconfigured interfaces
		if len(iface.AddressMethod) == 0 {
			iface.AddressMethod = append(iface.AddressMethod, &address.DHCP{})
		}

		// Bring up the interface
		if err = n.ifup(iface.Index); err != nil {
			log.Printf("Failed to bring up %s: %v", iface.Name, err)
			continue
		}

		// Generate rtnetlink.AddressMessage for each address method defined on
		// the interface
		for _, method := range iface.AddressMethod {
			log.Printf("configuring %s addressing for %s\n", method.Name(), iface.Name)
			if err = n.configureInterface(method, iface.Name, iface.Index); err != nil {
				// Treat as non fatal error when failing to configure an interface
				log.Println(err)
				continue
			}

			// Aggregate a list of DNS servers/resolvers
			resolvers = append(resolvers, method.Resolvers()...)
		}
	}

	// Write out resolv.conf
	if err = writeResolvConf(resolvers); err != nil {
		return err
	}

	return nil
}

// Renew sets up a long running loop to refresh a network interfaces
// addressing configuration. Currently this only applies to interfaces
// configured by DHCP.
func (n *Networkd) Renew(ifaces ...*nic.NetworkInterface) {
	var wg sync.WaitGroup
	for _, iface := range ifaces {
		for _, method := range iface.AddressMethod {
			if method.TTL() == 0 {
				continue
			}
			wg.Add(1)

			go n.renew(method, iface.Name, iface.Index)
		}
	}

	// We dont ever wg.Done
	// because this should run forever
	// Probably a better way to do this
	wg.Wait()
}

// renew sets up the looping to ensure we keep the addressing information
// up to date. We attempt to do our first reconfiguration halfway through
// address TTL. If that fails, we'll continue to attempt to retry every
// halflife.
func (n *Networkd) renew(method address.Addressing, name string, index uint32) {
	renewDuration := method.TTL() / 2
	for {
		<-time.After(renewDuration)

		if err := n.configureInterface(method, name, index); err != nil {
			log.Printf("failed to renew interface address for %s: %v\n", name, err)
			renewDuration = (renewDuration / 2)
		} else {
			renewDuration = method.TTL() / 2
		}
	}
}

// configureInterface handles the actual address discovery mechanism and
// netlink interaction to configure the interface
func (n *Networkd) configureInterface(method address.Addressing, name string, index uint32) error {
	// TODO s/Discover/Something else/
	// TODO make context more relevant
	var err error
	if err = method.Discover(context.Background(), name); err != nil {
		// Right now this would only happen during dhcp discovery failure
		log.Printf("Failed to prep %s: %v", name, err)
		return err
	}

	// Netlink message generation
	msg := address.AddressMessage(method, index)

	// Add address if not exist
	if err = n.AddressAdd(msg); err != nil {
		// TODO how do we want to handle failures in addressing?
		log.Printf("Failed to add address %+v to %s: %v", msg, name, err)
		return err
	}

	// Set link MTU if we got a response
	if err = n.setMTU(index, method.MTU()); err != nil {
		log.Printf("Failed to set mtu %d for %s: %v", method.MTU(), name, err)
		return err
	}

	// Add any routes
	rMsgs := address.RouteMessage(method, index)
	for _, r := range rMsgs {
		if err = n.RouteAdd(r); err != nil {
			// TODO how do we want to handle failures in routing?
			log.Printf("Failed to add route %+v for %s: %v", r, name, err)
			continue
		}
	}

	return err
}

// Hostname returns the first hostname found from the addressing methods.
func (n *Networkd) Hostname(ifaces ...*nic.NetworkInterface) string {
	for _, iface := range ifaces {
		for _, method := range iface.AddressMethod {
			if method.Hostname() != "" {
				return method.Hostname()
			}
		}
	}

	return ""
}

/*
// TODO add this in with some debug level of loggin
func (n *Networkd) printState() {
	rl, err := n.Conn.Route.List()
	if err != nil {
		log.Println(err)
		return
	}
	for _, r := range rl {
		log.Printf("%+v", r)
	}

	links, err := n.Conn.Link.List()
	if err != nil {
		log.Println(err)
		return
	}
	for _, link := range links {
		log.Printf("%+v", link)
		log.Printf("%+v", link.Attributes)
	}

	b, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("resolv.conf: %s", string(b))
}
*/
