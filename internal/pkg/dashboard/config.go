// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const (
	modeNoData = "No data"
	modeDHCP   = "DHCP"
	modeStatic = "Static"
)

type networkConfigFormData struct {
	hostname    string
	dnsServers  string
	timeServers string
	iface       string
	mode        string
	addresses   string
	gateway     string
}

//nolint:gocyclo
func (formData *networkConfigFormData) toPlatformNetworkConfig() (*runtime.PlatformNetworkConfig, error) {
	if formData.mode == modeNoData {
		return nil, fmt.Errorf("no data")
	}

	linkName := strings.TrimSpace(formData.iface)
	if linkName == "" {
		return nil, fmt.Errorf("no interface")
	}

	config := &runtime.PlatformNetworkConfig{
		Links: []network.LinkSpecSpec{
			{
				Name:        linkName,
				Logical:     false,
				Up:          true,
				Type:        nethelpers.LinkEther,
				ConfigLayer: network.ConfigOperator,
			},
		},
	}

	if formData.hostname != "" {
		config.Hostnames = []network.HostnameSpecSpec{
			{
				Hostname:    formData.hostname,
				Domainname:  "",
				ConfigLayer: network.ConfigOperator,
			},
		}
	}

	dnsServers, err := formData.parseAddresses(formData.dnsServers)
	if err != nil {
		return nil, err
	}

	if len(dnsServers) > 0 {
		config.Resolvers = []network.ResolverSpecSpec{
			{
				DNSServers:  dnsServers,
				ConfigLayer: network.ConfigOperator,
			},
		}
	}

	timeServers := formData.parseHosts(formData.timeServers)

	if len(timeServers) > 0 {
		config.TimeServers = []network.TimeServerSpecSpec{
			{
				NTPServers:  timeServers,
				ConfigLayer: network.ConfigOperator,
			},
		}
	}

	if formData.mode == modeDHCP {
		config.Operators = []network.OperatorSpecSpec{
			{
				Operator:  network.OperatorDHCP4, // TODO(dashboard): how do we decide if it's DHCP4 or DHCP6?
				LinkName:  linkName,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: 1024,
				},
				DHCP6:       network.DHCP6OperatorSpec{},
				ConfigLayer: network.ConfigOperator,
			},
		}
	} else if formData.mode == modeStatic {
		config.Addresses, err = formData.buildAddresses(linkName)
		if err != nil {
			return nil, err
		}

		config.Routes, err = formData.buildRoutes(linkName)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

func (formData *networkConfigFormData) parseAddresses(text string) ([]netip.Addr, error) {
	split := strings.Split(text, ",")
	addresses := make([]netip.Addr, 0, len(split))

	for _, address := range split {
		trimmed := strings.TrimSpace(address)
		if trimmed == "" {
			continue
		}

		addr, err := netip.ParseAddr(trimmed)
		if err != nil {
			return nil, err
		}

		addresses = append(addresses, addr)
	}

	return addresses, nil
}

func (formData *networkConfigFormData) parseHosts(text string) []string {
	split := strings.Split(text, ",")
	hosts := make([]string, 0, len(split))

	for _, host := range split {
		trimmed := strings.TrimSpace(host)
		if trimmed == "" {
			continue
		}

		hosts = append(hosts, trimmed)
	}

	return hosts
}

func (formData *networkConfigFormData) buildAddresses(linkName string) ([]network.AddressSpecSpec, error) {
	addressesSplit := strings.Split(formData.addresses, ",")
	addresses := make([]network.AddressSpecSpec, 0, len(addressesSplit))

	for _, address := range addressesSplit {
		trimmed := strings.TrimSpace(address)
		if trimmed == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(trimmed)
		if err != nil {
			return nil, err
		}

		ipFamily := nethelpers.FamilyInet4
		if prefix.Addr().Is6() {
			ipFamily = nethelpers.FamilyInet6
		}

		addresses = append(addresses, network.AddressSpecSpec{
			Address:     prefix,
			LinkName:    linkName,
			Family:      ipFamily,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigOperator,
		})
	}

	return addresses, nil
}

func (formData *networkConfigFormData) buildRoutes(linkName string) ([]network.RouteSpecSpec, error) {
	gateway := strings.TrimSpace(formData.gateway)

	if gateway == "" {
		return nil, fmt.Errorf("no gateway")
	}

	gatewayAddr, err := netip.ParseAddr(gateway)
	if err != nil {
		return nil, err
	}

	family := nethelpers.FamilyInet4
	if gatewayAddr.Is6() {
		family = nethelpers.FamilyInet6
	}

	return []network.RouteSpecSpec{
		{
			Family:      family,
			Gateway:     gatewayAddr,
			OutLinkName: linkName,
			Table:       nethelpers.TableMain,
			Scope:       nethelpers.ScopeGlobal,
			Type:        nethelpers.TypeUnicast,
			Protocol:    nethelpers.ProtocolStatic,
			ConfigLayer: network.ConfigOperator,
		},
	}, nil
}
