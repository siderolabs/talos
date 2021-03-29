// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/AlekSi/pointer"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// NetworkConfigOption generates NetworkConfig.
type NetworkConfigOption func(machine.Type, *NetworkConfig) error

// WithNetworkConfig sets whole network config structure, overwrites any previous options.
func WithNetworkConfig(c *NetworkConfig) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		*cfg = *c

		return nil
	}
}

// WithNetworkNameservers sets global nameservers list.
func WithNetworkNameservers(nameservers ...string) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		cfg.NameServers = append(cfg.NameServers, nameservers...)

		return nil
	}
}

// WithNetworkInterfaceIgnore marks interface as ignored.
func WithNetworkInterfaceIgnore(iface string) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		cfg.getDevice(iface).DeviceIgnore = true

		return nil
	}
}

// WithNetworkInterfaceDHCP enables DHCP for the interface.
func WithNetworkInterfaceDHCP(iface string, enable bool) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		cfg.getDevice(iface).DeviceDHCP = true

		return nil
	}
}

// WithNetworkInterfaceDHCPv4 enables DHCPv4 for the interface.
func WithNetworkInterfaceDHCPv4(iface string, enable bool) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		dev := cfg.getDevice(iface)

		if dev.DeviceDHCPOptions == nil {
			dev.DeviceDHCPOptions = &DHCPOptions{}
		}

		dev.DeviceDHCPOptions.DHCPIPv4 = pointer.ToBool(enable)

		return nil
	}
}

// WithNetworkInterfaceDHCPv6 enables DHCPv6 for the interface.
func WithNetworkInterfaceDHCPv6(iface string, enable bool) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		dev := cfg.getDevice(iface)

		if dev.DeviceDHCPOptions == nil {
			dev.DeviceDHCPOptions = &DHCPOptions{}
		}

		dev.DeviceDHCPOptions.DHCPIPv6 = pointer.ToBool(enable)

		return nil
	}
}

// WithNetworkInterfaceCIDR configures interface for static addressing.
func WithNetworkInterfaceCIDR(iface, cidr string) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		cfg.getDevice(iface).DeviceCIDR = cidr

		return nil
	}
}

// WithNetworkInterfaceMTU configures interface MTU.
func WithNetworkInterfaceMTU(iface string, mtu int) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		cfg.getDevice(iface).DeviceMTU = mtu

		return nil
	}
}

// WithNetworkInterfaceWireguard configures interface for Wireguard.
func WithNetworkInterfaceWireguard(iface string, wireguardConfig *DeviceWireguardConfig) NetworkConfigOption {
	return func(_ machine.Type, cfg *NetworkConfig) error {
		cfg.getDevice(iface).DeviceWireguardConfig = wireguardConfig

		return nil
	}
}

// WithNetworkInterfaceVirtualIP configures interface for Virtual IP.
func WithNetworkInterfaceVirtualIP(iface, cidr string) NetworkConfigOption {
	return func(machineType machine.Type, cfg *NetworkConfig) error {
		if machineType == machine.TypeJoin {
			return nil
		}

		cfg.getDevice(iface).DeviceVIPConfig = &DeviceVIPConfig{
			SharedIP: cidr,
		}

		return nil
	}
}
