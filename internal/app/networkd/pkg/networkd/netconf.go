/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"net"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/config"
)

// NetConf provides a mapping between an interface link and the functional
// options needed to configure the interface
type NetConf map[*net.Interface][]nic.Option

// BuildOptions translates the supplied config to functional options.
func (n *NetConf) BuildOptions(config config.Configurator) error {
	for link, opts := range *n {
		for _, device := range config.Machine().Network().Devices() {
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
