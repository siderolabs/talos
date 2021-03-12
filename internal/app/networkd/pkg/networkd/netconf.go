// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// buildOptions translates the supplied config to nic.Option used for
// configuring the interface.
//nolint:gocyclo,cyclop
func buildOptions(logger *log.Logger, device config.Device, hostname string) (name string, opts []nic.Option, err error) {
	opts = append(opts, nic.WithName(device.Interface()))

	if device.Ignore() || procfs.ProcCmdline().Get(constants.KernelParamNetworkInterfaceIgnore).Contains(device.Interface()) {
		opts = append(opts, nic.WithIgnore())

		return device.Interface(), opts, err
	}

	// Configure Addressing
	switch {
	case device.CIDR() != "":
		s := &address.Static{CIDR: device.CIDR(), RouteList: device.Routes(), Mtu: device.MTU()}

		// Set a default for the hostname to ensure we always have a valid
		// ip + hostname pair
		ip := s.Address().IP.String()
		s.FQDN = fmt.Sprintf("%s-%s", "talos", strings.ReplaceAll(ip, ".", "-"))

		if hostname != "" {
			s.FQDN = hostname
		}

		opts = append(opts, nic.WithAddressing(s))
	case device.DHCP():
		if device.DHCPOptions().IPv4() {
			d := &address.DHCP4{DHCPOptions: device.DHCPOptions(), RouteList: device.Routes(), Mtu: device.MTU()}
			opts = append(opts, nic.WithAddressing(d))
		}

		if device.DHCPOptions().IPv6() {
			d := &address.DHCP6{Mtu: device.MTU()}
			opts = append(opts, nic.WithAddressing(d))
		}
	default:
		// Allow master interface without any addressing if VLANs exist
		if len(device.Vlans()) > 0 {
			logger.Printf("no addressing for master device %s", device.Interface())

			opts = append(opts, nic.WithNoAddressing())
		} else {
			// No CIDR and DHCP==false results in a static without an IP.
			// This handles cases like slaac addressing.
			s := &address.Static{RouteList: device.Routes(), Mtu: device.MTU()}
			opts = append(opts, nic.WithAddressing(s))
		}
	}

	// Configure Vlan interfaces
	for _, vlan := range device.Vlans() {
		opts = append(opts, nic.WithVlan(vlan.ID()))
		if vlan.CIDR() != "" {
			opts = append(opts, nic.WithVlanCIDR(vlan.ID(), vlan.CIDR(), vlan.Routes()))
		}

		if vlan.DHCP() {
			opts = append(opts, nic.WithVlanDhcp(vlan.ID()))
		}
	}

	// Handle dummy interface
	if device.Dummy() {
		opts = append(opts, nic.WithDummy())
	}

	if device.WireguardConfig() != nil {
		opts = append(opts, nic.WithWireguardConfig(device.WireguardConfig()))
	}

	if device.VIPConfig() != nil {
		opts = append(opts, nic.WithVIPConfig(device.VIPConfig()))
	}

	// Configure Bonding
	if device.Bond() == nil {
		return device.Interface(), opts, err
	}

	opts = append(opts, nic.WithBond(true))

	if len(device.Bond().Interfaces()) == 0 {
		return device.Interface(), opts, fmt.Errorf("invalid bond configuration for %s: must supply sub interfaces for bonded interface", device.Interface())
	}

	opts = append(opts, nic.WithSubInterface(device.Bond().Interfaces()...))

	if device.Bond().Mode() != "" {
		opts = append(opts, nic.WithBondMode(device.Bond().Mode()))
	}

	if device.Bond().HashPolicy() != "" {
		opts = append(opts, nic.WithHashPolicy(device.Bond().HashPolicy()))
	}

	if device.Bond().LACPRate() != "" {
		opts = append(opts, nic.WithLACPRate(device.Bond().LACPRate()))
	}

	if device.Bond().MIIMon() > 0 {
		opts = append(opts, nic.WithMIIMon(device.Bond().MIIMon()))
	}

	if device.Bond().UpDelay() > 0 {
		opts = append(opts, nic.WithUpDelay(device.Bond().UpDelay()))
	}

	if device.Bond().DownDelay() > 0 {
		opts = append(opts, nic.WithDownDelay(device.Bond().DownDelay()))
	}

	if !device.Bond().UseCarrier() {
		opts = append(opts, nic.WithUseCarrier(device.Bond().UseCarrier()))
	}

	if device.Bond().ARPInterval() > 0 {
		opts = append(opts, nic.WithARPInterval(device.Bond().ARPInterval()))
	}

	// if device.Bond.ARPIPTarget {
	//	opts = append(opts, nic.WithARPIPTarget(device.Bond.ARPIPTarget))
	//}

	if device.Bond().ARPValidate() != "" {
		opts = append(opts, nic.WithARPValidate(device.Bond().ARPValidate()))
	}

	if device.Bond().ARPAllTargets() != "" {
		opts = append(opts, nic.WithARPAllTargets(device.Bond().ARPAllTargets()))
	}

	if device.Bond().Primary() != "" {
		opts = append(opts, nic.WithPrimary(device.Bond().Primary()))
	}

	if device.Bond().PrimaryReselect() != "" {
		opts = append(opts, nic.WithPrimaryReselect(device.Bond().PrimaryReselect()))
	}

	if device.Bond().FailOverMac() != "" {
		opts = append(opts, nic.WithFailOverMAC(device.Bond().FailOverMac()))
	}

	if device.Bond().ResendIGMP() > 0 {
		opts = append(opts, nic.WithResendIGMP(device.Bond().ResendIGMP()))
	}

	if device.Bond().NumPeerNotif() > 0 {
		opts = append(opts, nic.WithNumPeerNotif(device.Bond().NumPeerNotif()))
	}

	if device.Bond().AllSlavesActive() > 0 {
		opts = append(opts, nic.WithAllSlavesActive(device.Bond().AllSlavesActive()))
	}

	if device.Bond().MinLinks() > 0 {
		opts = append(opts, nic.WithMinLinks(device.Bond().MinLinks()))
	}

	if device.Bond().LPInterval() > 0 {
		opts = append(opts, nic.WithLPInterval(device.Bond().LPInterval()))
	}

	if device.Bond().PacketsPerSlave() > 0 {
		opts = append(opts, nic.WithPacketsPerSlave(device.Bond().PacketsPerSlave()))
	}

	if device.Bond().ADSelect() != "" {
		opts = append(opts, nic.WithADSelect(device.Bond().ADSelect()))
	}

	if device.Bond().ADActorSysPrio() > 0 {
		opts = append(opts, nic.WithADActorSysPrio(device.Bond().ADActorSysPrio()))
	}

	if device.Bond().ADUserPortKey() > 0 {
		opts = append(opts, nic.WithADUserPortKey(device.Bond().ADUserPortKey()))
	}

	// if device.Bond.ADActorSystem != "" {
	//	opts = append(opts, nic.WithADActorSystem(device.Bond.ADActorSystem))
	//}

	if device.Bond().TLBDynamicLB() > 0 {
		opts = append(opts, nic.WithTLBDynamicLB(device.Bond().TLBDynamicLB()))
	}

	if device.Bond().PeerNotifyDelay() > 0 {
		opts = append(opts, nic.WithPeerNotifyDelay(device.Bond().PeerNotifyDelay()))
	}

	return device.Interface(), opts, err
}

//nolint:gocyclo
func buildKernelOptions(cmdline string) (name string, opts []nic.Option) {
	// https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt
	// ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>:<ntp0-ip>
	fields := strings.Split(cmdline, ":")

	// If dhcp is specified, we'll handle it as a normal discovered
	// interface
	if len(fields) == 1 && fields[0] == "dhcp" {
		return name, opts
	}

	// If there are not enough fields specified, we'll bail
	if len(fields) < 4 {
		return name, opts
	}

	var (
		device    = &v1alpha1.Device{}
		hostname  string
		link      *net.Interface
		resolvers = []net.IP{}
	)

	for idx, field := range fields {
		switch idx {
		// Address
		case 0:
			device.DeviceCIDR = field
		// NFS Server
		// case 1:
		// Gateway
		case 2:
			device.DeviceRoutes = []*v1alpha1.Route{
				{
					RouteNetwork: "0.0.0.0/0",
					RouteGateway: field,
				},
			}
		// Netmask
		case 3:
			mask := net.ParseIP(field).To4()
			ipmask := net.IPv4Mask(mask[0], mask[1], mask[2], mask[3])
			ones, _ := ipmask.Size()
			device.DeviceCIDR = fmt.Sprintf("%s/%d", device.CIDR(), ones)
		// Hostname
		case 4:
			hostname = field
		// Interface name
		case 5:
			iface, err := net.InterfaceByName(field)
			if err == nil {
				link = iface
			}
		// Configuration method
		// case 6:
		// Primary DNS Resolver
		case 7:
			fallthrough
		// Secondary DNS Resolver
		case 8:
			nameserverIP := net.ParseIP(field)
			if nameserverIP != nil {
				resolvers = append(resolvers, nameserverIP)
			}
		}
	}
	// NTP server
	// case 9:
	// 	// k.NTPServer = field

	// Find the first non-loopback interface
	if link == nil {
		ifaces, err := net.Interfaces()
		if err != nil {
			return hostname, opts
		}

		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			i := iface

			link = &i

			break
		}
	}

	if device.DeviceInterface == "" {
		opts = append(opts, nic.WithName(link.Name))
	}

	routes := make([]config.Route, len(device.DeviceRoutes))

	for i := 0; i < len(device.DeviceRoutes); i++ {
		routes[i] = device.DeviceRoutes[i]
	}

	s := &address.Static{Mtu: device.DeviceMTU, NameServers: resolvers, FQDN: hostname, NetIf: link, CIDR: device.DeviceCIDR, RouteList: routes}
	opts = append(opts, nic.WithAddressing(s))

	return link.Name, opts
}
