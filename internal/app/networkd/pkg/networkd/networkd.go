/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"net"
	"strings"

	"github.com/jsimonetti/rtnetlink/rtnl"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/userdata"
)

type networkd struct {
	Conn             *rtnl.Conn
	Interfaces       []*NetworkInterface
	DefaultInterface string
}

type NetworkInterface struct {
	Name          string
	Link          *net.Interface
	AddressConfig AddressConfig
	Routes        []userdata.Route
	// TODO need something in here for bonding config
}

func New() (nwd *networkd, err error) {
	conn, err := rtnl.Dial(nil)
	if err != nil {
		return nwd, err
	}

	// Set up a default interface to bootstrap with.
	// Used for pulling initial userdata.
	defaultInterface := "eth0"

	if option := kernel.Cmdline().Get(constants.KernelParamDefaultInterface).First(); option != nil {
		defaultInterface = *option
	}

	nwd = &networkd{
		Conn:             conn,
		DefaultInterface: defaultInterface,
	}

	err = nwd.discover()

	return nwd, err
}

func (n *networkd) discover() (err error) {
	// Discover local interfaces
	var links []*net.Interface
	links, err = n.Conn.Links()
	if err != nil {
		return err
	}

	n.Interfaces = make([]*NetworkInterface, len(links))
	for idx, link := range links {
		n.Interfaces[idx] = &NetworkInterface{
			Name: link.Name,
			Link: link,
		}
	}

	return err
}

// Parse merges the passed in userdata with the locally discovered
// network interfaces and defines the configuration for the interface
func (n *networkd) Parse(data *userdata.UserData) (err error) {

	// Skip out on any custom network configuration if
	// not specified
	if !validNetworkUserData(data) {
		return err
	}

	// Add any bond interfaces
	for _, netifconf := range data.Networking.OS.Devices {
		// Just going to gauge by interface name for now
		// can probably reaffirm with BondConfig ( whatever
		// that ends up looking like )
		if strings.HasPrefix(netifconf.Interface, "bond") {
			n.Interfaces = append(n.Interfaces, &NetworkInterface{
				Name: netifconf.Interface,
			})
		}
	}

	if err = n.configureAddressing(data); err != nil {
		return err
	}

	n.configureRoutes(data)

	return err
}

func validNetworkUserData(data *userdata.UserData) bool {
	if data == nil {
		return false
	}

	if data.Networking == nil {
		return false
	}

	if data.Networking.OS == nil {
		return false
	}

	if data.Networking.OS.Devices == nil {
		return false
	}

	return true
}

func (n *networkd) configureAddressing(data *userdata.UserData) (err error) {
	// Handle mapping config defined in userdata to local interface
	// configuration
	for _, netifconf := range data.Networking.OS.Devices {
		for _, netif := range n.Interfaces {
			if netifconf.Interface == netif.Name {
				var ac AddressConfig
				ac, err = NewAddress(netifconf)
				if err != nil {
					return err
				}

				netif.AddressConfig = ac

				break
			}
		}
	}

	return err
}

func (n *networkd) configureRoutes(data *userdata.UserData) {
	// Handle mapping config defined in userdata to local interface
	// configuration
	for _, netifconf := range data.Networking.OS.Devices {
		if len(netifconf.Routes) == 0 {
			continue
		}

		for _, netif := range n.Interfaces {
			if netifconf.Interface == netif.Name {
				netif.Routes = netifconf.Routes
				break
			}
		}
	}
}
