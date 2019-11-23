// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nic

import (
	"errors"
	"net"

	"github.com/hashicorp/go-multierror"
	"github.com/mdlayher/netlink"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/constants"
)

const (
	// https://tools.ietf.org/html/rfc791
	MinimumMTU = 68
	MaximumMTU = 65536
)

// NetworkInterface provides an abstract configuration representation for a
// network interface
type NetworkInterface struct {
	Ignore        bool
	Name          string
	Bonded        bool
	MTU           uint32
	Index         uint32
	SubInterfaces []*net.Interface
	AddressMethod []address.Addressing
	BondSettings  []netlink.Attribute
}

// IsIgnored checks the network interface to see if it should be ignored and not configured
func (n *NetworkInterface) IsIgnored() bool {
	if n.Ignore || kernel.ProcCmdline().Get(constants.KernelParamNetworkInterfaceIgnore).Contains(n.Name) {
		return true
	}

	return false
}

// Create returns a NetworkInterface with all of the given setter options
// applied
func Create(setters ...Option) (*NetworkInterface, error) {
	// Default interface setup
	iface := defaultOptions()

	// Configure interface with any specified options
	var result *multierror.Error
	for _, setter := range setters {
		result = multierror.Append(setter(iface))
	}

	// TODO: May need to look at switching this around to filter by Interface.HardwareAddr
	// Ensure we have an interface name defined
	if iface.Name == "" {
		result = multierror.Append(errors.New("interface must have a name"))
	}

	// If no addressing methods have been configured, default to DHCP
	// TODO: do we want this behavior or to be explicit with config
	// so we dont configure every interface be default?
	if len(iface.AddressMethod) == 0 && iface.Name != "" {
		link, err := net.InterfaceByName(iface.Name)
		if err != nil {
			result = multierror.Append(err)
		}

		if link != nil {
			iface.AddressMethod = append(iface.AddressMethod, &address.DHCP{NetIf: link})
		}
	}

	return iface, result.ErrorOrNil()
}
