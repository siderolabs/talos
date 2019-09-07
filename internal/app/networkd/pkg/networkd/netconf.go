/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"net"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/userdata"
)

// NetConf provides a mapping between an interface link and the functional
// options needed to configure the interface
type NetConf map[*net.Interface][]nic.Option

// OverlayUserData translates the supplied userdata to functional options
func (n *NetConf) OverlayUserData(data *userdata.UserData) error {
	if !validateNetworkUserData(data) {
		return nil
	}

	for link, opts := range *n {
		for _, device := range data.Networking.OS.Devices {

			device := device
			if link.Name != device.Interface {
				continue
			}

			if device.Ignore {
				(*n)[link] = append(opts, nic.WithIgnore())
				continue
			}

			// Configure Addressing
			if device.DHCP {
				d := &address.DHCP{NetIf: link}
				(*n)[link] = append(opts, nic.WithAddressing(d))
			}

			if device.CIDR != "" {
				s := &address.Static{Device: &device, NetIf: link}
				(*n)[link] = append(opts, nic.WithAddressing(s))
			}

			// Configure MTU
			if device.MTU != 0 {
				(*n)[link] = append(opts, nic.WithMTU(uint32(device.MTU)))
			}

		}
	}

	return nil
}

// validateNetworkUserData ensures that we have actual data to do our lookups
func validateNetworkUserData(data *userdata.UserData) bool {
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
