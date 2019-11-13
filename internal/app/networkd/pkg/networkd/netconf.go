// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"context"
	"net"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// NetConf provides a mapping between an interface link and the functional
// options needed to configure the interface
type NetConf map[*net.Interface][]nic.Option

// BuildOptionsFromConfig translates the supplied config to functional options.
func (n *NetConf) BuildOptionsFromConfig(config runtime.Configurator) error {
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
				s := &address.Static{Device: &device, NetIf: link, NameServers: config.Machine().Network().Resolvers()}
				(*n)[link] = append(opts, nic.WithAddressing(s))
			}
		}
	}

	return nil
}

// BuildOptionsFromKernel translates the supplied config to functional options.
func (n *NetConf) BuildOptionsFromKernel() error {
	// Check to see if a kernel supplied configuration option was specified
	kern := &address.Kernel{}
	if err := kern.Discover(context.Background()); err != nil {
		return nil
	}

	for link, opts := range *n {
		if link.Name != kern.Device {
			continue
		}

		(*n)[link] = append(opts, nic.WithAddressing(kern))
	}

	return nil
}
