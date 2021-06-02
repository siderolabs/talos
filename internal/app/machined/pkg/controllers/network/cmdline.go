// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// CmdlineNetworking contains parsed cmdline networking settings.
type CmdlineNetworking struct {
	DHCP             bool
	Address          netaddr.IPPrefix
	Gateway          netaddr.IP
	Hostname         string
	LinkName         string
	DNSAddresses     []netaddr.IP
	NTPAddresses     []netaddr.IP
	IgnoreInterfaces []string
}

// ParseCmdlineNetwork parses `ip=` and Talos specific kernel cmdline argument producing all the available configuration options.
//
//nolint:gocyclo,cyclop
func ParseCmdlineNetwork(cmdline *procfs.Cmdline) (CmdlineNetworking, error) {
	var (
		settings CmdlineNetworking
		err      error
	)

	// process Talos specific kernel params
	cmdlineHostname := cmdline.Get(constants.KernelParamHostname).First()
	if cmdlineHostname != nil {
		settings.Hostname = *cmdlineHostname
	}

	ignoreInterfaces := cmdline.Get(constants.KernelParamNetworkInterfaceIgnore)
	for i := 0; ignoreInterfaces.Get(i) != nil; i++ {
		settings.IgnoreInterfaces = append(settings.IgnoreInterfaces, *ignoreInterfaces.Get(i))
	}

	// standard ip=
	ipSettings := cmdline.Get("ip").First()
	if ipSettings == nil {
		return settings, nil
	}

	// https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt
	// ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>:<ntp0-ip>
	fields := strings.Split(*ipSettings, ":")

	// If dhcp is specified, we'll handle it as a normal discovered
	// interface
	if len(fields) == 1 && fields[0] == "dhcp" {
		settings.DHCP = true
	}

	if !settings.DHCP {
		for i := range fields {
			if fields[i] == "" {
				continue
			}

			switch i {
			case 0:
				settings.Address.IP, err = netaddr.ParseIP(fields[0])
				if err != nil {
					return settings, fmt.Errorf("cmdline address parse failure: %s", err)
				}

				// default is to have complete address masked
				settings.Address.Bits = settings.Address.IP.BitLen()
			case 2:
				settings.Gateway, err = netaddr.ParseIP(fields[2])
				if err != nil {
					return settings, fmt.Errorf("cmdline gateway parse failure: %s", err)
				}
			case 3:
				var netmask netaddr.IP

				netmask, err = netaddr.ParseIP(fields[3])
				if err != nil {
					return settings, fmt.Errorf("cmdline netmask parse failure: %s", err)
				}

				ones, _ := net.IPMask(netmask.IPAddr().IP).Size()

				settings.Address.Bits = uint8(ones)
			case 4:
				if settings.Hostname == "" {
					settings.Hostname = fields[4]
				}
			case 5:
				settings.LinkName = fields[5]
			case 7, 8:
				var dnsIP netaddr.IP

				dnsIP, err = netaddr.ParseIP(fields[i])
				if err != nil {
					return settings, fmt.Errorf("error parsing DNS IP: %w", err)
				}

				settings.DNSAddresses = append(settings.DNSAddresses, dnsIP)
			case 9:
				var ntpIP netaddr.IP

				ntpIP, err = netaddr.ParseIP(fields[i])
				if err != nil {
					return settings, fmt.Errorf("error parsing DNS IP: %w", err)
				}

				settings.NTPAddresses = append(settings.NTPAddresses, ntpIP)
			}
		}
	}

	// if interface name is not set, pick the first non-loopback interface
	if settings.LinkName == "" {
		ifaces, _ := net.Interfaces() //nolint:errcheck // ignoring error here as ifaces will be empty

		sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Name < ifaces[j].Name })

		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			settings.LinkName = iface.Name

			break
		}
	}

	return settings, nil
}
