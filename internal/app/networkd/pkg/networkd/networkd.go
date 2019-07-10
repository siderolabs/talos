/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"net"
	"strings"
	"sync"

	"github.com/jsimonetti/rtnetlink/rtnl"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/userdata"
)

type networkd struct {
	conn             *rtnl.Conn
	interfaces       []*NetworkInterface
	defaultInterface string

	mu sync.Mutex
}

type NetworkInterface struct {
	Name          string
	Link          *net.Interface
	AddressConfig AddressConfig
	Routes        []userdata.Route
	// TODO need something in here for bonding config
}

// Representation of internal network state
var state *networkd

func New() (err error) {

	// Set up a default interface to bootstrap with.
	// Used for pulling initial userdata.
	// This can be overridden by the kernel command line
	defaultInterface := "eth0"

	if option := kernel.Cmdline().Get(constants.KernelParamDefaultInterface).First(); option != nil {
		defaultInterface = *option
	}

	// Netlink connection
	conn, err := rtnl.Dial(nil)
	if err != nil {
		return err
	}

	// Initialize our internal state
	state = &networkd{
		conn:             conn,
		defaultInterface: defaultInterface,
	}

	// Discover local interfaces
	err = state.discover()

	return err
}

func (n *networkd) discover() (err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	var links []*net.Interface
	links, err = n.conn.Links()
	if err != nil {
		return err
	}

	n.interfaces = make([]*NetworkInterface, len(links))
	for idx, link := range links {
		n.interfaces[idx] = &NetworkInterface{
			Name: link.Name,
			Link: link,
		}
	}

	return err
}

// Parse merges the passed in userdata with the locally discovered
// network interfaces and defines the configuration for the interface
func Parse(data *userdata.UserData) (err error) {

	// Skip out on any custom network configuration if
	// not specified
	if !validNetworkUserData(data) {
		return err
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	// Add any bond interfaces
	for _, netifconf := range data.Networking.OS.Devices {
		// Just going to gauge by interface name for now
		// can probably reaffirm with BondConfig ( whatever
		// that ends up looking like )
		if strings.HasPrefix(netifconf.Interface, "bond") {
			state.interfaces = append(state.interfaces, &NetworkInterface{
				Name: netifconf.Interface,
			})
		}
	}

	if err = state.configureAddressing(data); err != nil {
		return err
	}

	state.configureRoutes(data)

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
		for _, netif := range n.interfaces {
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

		for _, netif := range n.interfaces {
			if netifconf.Interface == netif.Name {
				netif.Routes = netifconf.Routes
				break
			}
		}
	}
}

func (n *networkd) Interfaces() []*NetworkInterface {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.interfaces
}
