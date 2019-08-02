/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/userdata"
)

// NetConf provides a mapping between an interface name and the functional
// options needed to configure the interface
type NetConf map[string][]nic.Option

// OverlayUserData translates the supplied userdata to functional options
func (n *NetConf) OverlayUserData(data *userdata.UserData) error {
	if !validNetworkUserData(data) {
		return nil
	}

	for name, opts := range *n {
		for _, device := range data.Networking.OS.Devices {
			device := device
			if name != device.Interface {
				continue
			}

			if device.CIDR != "" {
				s := &address.Static{Device: &device}

				(*n)[name] = append(opts, nic.WithAddressing(s))
			}

			// Configure MTU
			if device.MTU != 0 {
				(*n)[name] = append(opts, nic.WithMTU(uint32(device.MTU)))
			}
		}
	}

	return nil
}

// validateNetworkUserData ensures that we have actual data to do our lookups
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
