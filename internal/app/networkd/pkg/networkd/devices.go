// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"net"

	networkapi "github.com/talos-systems/talos/pkg/machinery/api/network"
)

// GetDevices gathers information about existing network interfaces and their flags.
func GetDevices() (reply *networkapi.InterfacesResponse, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return reply, err
	}

	resp := &networkapi.Interfaces{}

	for _, iface := range ifaces {
		ifaceaddrs, err := iface.Addrs()
		if err != nil {
			return reply, err
		}

		addrs := make([]string, 0, len(ifaceaddrs))
		for _, addr := range ifaceaddrs {
			addrs = append(addrs, addr.String())
		}

		ifmsg := &networkapi.Interface{
			Index:        uint32(iface.Index),
			Mtu:          uint32(iface.MTU),
			Name:         iface.Name,
			Hardwareaddr: iface.HardwareAddr.String(),
			Flags:        networkapi.InterfaceFlags(iface.Flags),
			Ipaddress:    addrs,
		}

		resp.Interfaces = append(resp.Interfaces, ifmsg)
	}

	return &networkapi.InterfacesResponse{
		Messages: []*networkapi.Interfaces{
			resp,
		},
	}, nil
}
