// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nic

import (
	"fmt"
	"net"

	"github.com/mdlayher/netlink"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/pkg/machinery/config"
)

// Vlan contins interface related parameters to a VLAN device.
type Vlan struct {
	Parent        string
	ID            uint16
	Link          *net.Interface
	VlanSettings  *netlink.AttributeEncoder
	AddressMethod []address.Addressing
}

// WithVlan defines the VLAN id to use.
func WithVlan(id uint16) Option {
	return func(n *NetworkInterface) (err error) {
		for _, vlan := range n.Vlans {
			if vlan.ID == id {
				return fmt.Errorf("duplicate VLAN id  %v given", vlan)
			}
		}

		vlan := &Vlan{
			ID:           id,
			VlanSettings: netlink.NewAttributeEncoder(),
		}

		vlan.VlanSettings.Uint16(uint16(IFLA_VLAN_ID), vlan.ID)
		n.Vlans = append(n.Vlans, vlan)

		return nil
	}
}

// WithVlanDhcp sets a VLAN device with DHCP.
func WithVlanDhcp(id uint16) Option {
	return func(n *NetworkInterface) (err error) {
		for _, vlan := range n.Vlans {
			if vlan.ID == id {
				vlan.AddressMethod = append(vlan.AddressMethod, &address.DHCP4{}) // TODO: should we enable DHCP6 by default?

				return nil
			}
		}

		return fmt.Errorf("VLAN id not found for DHCP. Vlan ID  %v given", id)
	}
}

// WithVlanCIDR defines if the interface have static CIDRs added.
func WithVlanCIDR(id uint16, cidr string, routeList []config.Route) Option {
	return func(n *NetworkInterface) (err error) {
		for _, vlan := range n.Vlans {
			if vlan.ID == id {
				vlan.AddressMethod = append(vlan.AddressMethod, &address.Static{CIDR: cidr, RouteList: routeList})

				return nil
			}
		}

		return fmt.Errorf("VLAN id not found for CIDR setting  %v given", id)
	}
}
