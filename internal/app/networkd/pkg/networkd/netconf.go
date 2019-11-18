// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"fmt"
	"net"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// NetConf provides a mapping between an interface link and the functional
// options needed to configure the interface
type NetConf map[*net.Interface][]nic.Option

// BuildOptions translates the supplied config to functional options.
func (n *NetConf) BuildOptions(config runtime.Configurator) error {
	for link, opts := range *n {
		for _, device := range config.Machine().Network().Devices() {
			device := device
			if link.Name != device.Interface {
				continue
			}

			if device.Ignore || kernel.ProcCmdline().Get(constants.KernelParamNetworkInterfaceIgnore).Contains(device.Interface) {
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

			// Configure Bonding
			if device.Bond == nil {
				continue
			}
			(*n)[link] = append(opts, nic.WithBond(true))

			if len(device.Bond.Interfaces) == 0 {
				return fmt.Errorf("invalid bond configuration: %s", "interfaces to be bonded must be supplied")
			}

			(*n)[link] = append(opts, nic.WithSubInterface(device.Bond.Interfaces...))

			if device.Bond.Mode != "" {
				(*n)[link] = append(opts, nic.WithSubInterface(device.Bond.Interfaces...))
			}
			if device.Bond.HashPolicy != "" {
				(*n)[link] = append(opts, nic.WithHashPolicy(device.Bond.HashPolicy))
			}
			if device.Bond.LACPRate != "" {
				(*n)[link] = append(opts, nic.WithLACPRate(device.Bond.LACPRate))
			}
			if device.Bond.MIIMon > 0 {
				(*n)[link] = append(opts, nic.WithMIIMon(device.Bond.MIIMon))
			}
			if device.Bond.UpDelay > 0 {
				(*n)[link] = append(opts, nic.WithUpDelay(device.Bond.UpDelay))
			}
			if device.Bond.DownDelay > 0 {
				(*n)[link] = append(opts, nic.WithDownDelay(device.Bond.DownDelay))
			}

		}
	}

	return nil
}
