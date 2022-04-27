// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/ordered"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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
	NetworkLinkSpecs []network.LinkSpecSpec
}

// splitIPArgument splits the `ip=` kernel argument honoring the IPv6 addresses in square brackets.
func splitIPArgument(val string) []string {
	var (
		squared, prev int
		parts         []string
	)

	for i, c := range val {
		switch c {
		case '[':
			squared++
		case ']':
			squared--
		case ':':
			if squared != 0 {
				continue
			}

			parts = append(parts, strings.Trim(val[prev:i], "[]"))
			prev = i + 1
		}
	}

	parts = append(parts, strings.Trim(val[prev:], "[]"))

	return parts
}

// ParseCmdlineNetwork parses `ip=` and Talos specific kernel cmdline argument producing all the available configuration options.
//
//nolint:gocyclo,cyclop
func ParseCmdlineNetwork(cmdline *procfs.Cmdline) (CmdlineNetworking, error) {
	var (
		settings      CmdlineNetworking
		err           error
		linkSpecSpecs []network.LinkSpecSpec
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

	// dracut bond=
	// ref: https://man7.org/linux/man-pages/man7/dracut.cmdline.7.html
	bondSettings := cmdline.Get(constants.KernelParamBonding).First()

	if ipSettings != nil {
		// https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt
		// ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>:<ntp0-ip>
		fields := splitIPArgument(*ipSettings)

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
					var ip netaddr.IP

					ip, err = netaddr.ParseIP(fields[0])
					if err != nil {
						return settings, fmt.Errorf("cmdline address parse failure: %s", err)
					}

					// default is to have complete address masked
					settings.Address = netaddr.IPPrefixFrom(ip, ip.BitLen())
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

					settings.Address = netaddr.IPPrefixFrom(settings.Address.IP(), uint8(ones))
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

		linkSpecSpecs = append(linkSpecSpecs, network.LinkSpecSpec{
			Name:        settings.LinkName,
			Up:          true,
			ConfigLayer: network.ConfigCmdline,
		})
	}

	if bondSettings != nil {
		var (
			bondName, bondMTU string
			bondSlaves        []string
			bondOptions       v1alpha1.Bond
		)

		// bond=<bondname>[:<bondslaves>:[:<options>[:<mtu>]]]
		fields := strings.Split(*bondSettings, ":")

		for i := range fields {
			if fields[i] == "" {
				continue
			}

			switch i {
			case 0:
				bondName = fields[0]
			case 1:
				bondSlaves = strings.Split(fields[1], ",")
			case 2:
				bondOptions, err = parseBondOptions(fields[2])
				if err != nil {
					return settings, err
				}
			case 3:
				bondMTU = fields[3]
			}
		}

		// set defaults as per https://man7.org/linux/man-pages/man7/dracut.cmdline.7.html
		// Talos by default sets bond mode to balance-rr
		if bondSlaves == nil {
			bondSlaves = []string{
				"eth0",
				"eth1",
			}
		}

		bondLinkSpec := network.LinkSpecSpec{
			Name:        bondName,
			Up:          true,
			ConfigLayer: network.ConfigCmdline,
		}

		if bondMTU != "" {
			mtu, err := strconv.Atoi(bondMTU)
			if err != nil {
				return settings, fmt.Errorf("error parsing bond MTU: %w", err)
			}

			bondLinkSpec.MTU = uint32(mtu)
		}

		if err := SetBondMaster(&bondLinkSpec, &bondOptions); err != nil {
			return settings, fmt.Errorf("error setting bond master: %w", err)
		}

		linkSpecSpecs = append(linkSpecSpecs, bondLinkSpec)

		for idx, slave := range bondSlaves {
			slaveLinkSpec := network.LinkSpecSpec{
				Name:        slave,
				Up:          true,
				ConfigLayer: network.ConfigCmdline,
			}
			SetBondSlave(&slaveLinkSpec, ordered.MakePair(bondName, idx))
			linkSpecSpecs = append(linkSpecSpecs, slaveLinkSpec)
		}
	}

	settings.NetworkLinkSpecs = linkSpecSpecs

	return settings, nil
}

// parseBondOptions parses the options string into v1alpha1.Bond
// v1alpha1.Bond was chosen to re-use the `SetBondMaster` and `SetBondSlave` functions
// ref: modinfo bonding
//nolint:gocyclo,cyclop
func parseBondOptions(options string) (v1alpha1.Bond, error) {
	var bond v1alpha1.Bond

	bondOptions := strings.Split(options, ",")

	for _, opt := range bondOptions {
		optionPair := strings.Split(opt, "=")

		switch optionPair[0] {
		case "arp_ip_target":
			bond.BondARPIPTarget = strings.Split(optionPair[1], ";")
		case "mode":
			bond.BondMode = optionPair[1]
		case "xmit_hash_policy":
			bond.BondHashPolicy = optionPair[1]
		case "lacp_rate":
			bond.BondLACPRate = optionPair[1]
		case "arp_validate":
			bond.BondARPValidate = optionPair[1]
		case "arp_all_targets":
			bond.BondARPAllTargets = optionPair[1]
		case "primary":
			bond.BondPrimary = optionPair[1]
		case "primary_reselect":
			bond.BondPrimaryReselect = optionPair[1]
		case "fail_over_mac":
			bond.BondFailOverMac = optionPair[1]
		case "ad_select":
			bond.BondADSelect = optionPair[1]
		case "miimon":
			miimon, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option miimon: %w", err)
			}

			bond.BondMIIMon = uint32(miimon)
		case "updelay":
			updelay, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option updelay: %w", err)
			}

			bond.BondUpDelay = uint32(updelay)
		case "downdelay":
			downdelay, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option downdelay: %w", err)
			}

			bond.BondDownDelay = uint32(downdelay)
		case "arp_interval":
			arpInterval, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option arp_interval: %w", err)
			}

			bond.BondARPInterval = uint32(arpInterval)
		case "resend_igmp":
			resendIGMP, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option resend_igmp: %w", err)
			}

			bond.BondResendIGMP = uint32(resendIGMP)
		case "min_links":
			minLinks, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option min_links: %w", err)
			}

			bond.BondMinLinks = uint32(minLinks)
		case "lp_interval":
			lpInterval, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option lp_interval: %w", err)
			}

			bond.BondLPInterval = uint32(lpInterval)
		case "packets_per_slave":
			packetsPerSlave, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option packets_per_slave: %w", err)
			}

			bond.BondPacketsPerSlave = uint32(packetsPerSlave)
		case "num_grat_arp":
			numGratArp, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option num_grat_arp: %w", err)
			}

			bond.BondNumPeerNotif = uint8(numGratArp)
		case "num_unsol_na":
			numGratArp, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option num_unsol_na: %w", err)
			}

			bond.BondNumPeerNotif = uint8(numGratArp)
		case "all_slaves_active":
			allSlavesActive, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option all_slaves_active: %w", err)
			}

			bond.BondAllSlavesActive = uint8(allSlavesActive)
		case "use_carrier":
			useCarrier, err := strconv.Atoi(optionPair[1])
			if err != nil {
				return bond, fmt.Errorf("error parsing bond option use_carrier: %w", err)
			}

			if useCarrier == 1 {
				val := []bool{true}
				bond.BondUseCarrier = &val[0]
			}
		default:
			return bond, fmt.Errorf("unknown bond option: %s", optionPair[0])
		}
	}

	return bond, nil
}
