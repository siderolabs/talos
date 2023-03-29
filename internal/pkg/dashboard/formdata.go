// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"fmt"
	"net/netip"
	"strings"
	"unicode"

	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const (
	interfaceNone = "(none)"

	modeDHCP   = "DHCP"
	modeStatic = "Static"
)

type networkConfigFormData struct {
	base        runtime.PlatformNetworkConfig
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
	var errs error

	config := &formData.base

	// zero-out the fields managed by the form
	config.Hostnames = nil
	config.Resolvers = nil
	config.TimeServers = nil
	config.Links = nil
	config.Operators = nil
	config.Addresses = nil
	config.Routes = nil

	if formData.hostname != "" {
		config.Hostnames = []network.HostnameSpecSpec{
			{
				Hostname:    formData.hostname,
				ConfigLayer: network.ConfigPlatform,
			},
		}
	}

	dnsServers, err := formData.parseAddresses(formData.dnsServers)
	if err != nil {
		errs = multierror.Append(errs, fmt.Errorf("failed to parse DNS servers: %w", err))
	}

	if len(dnsServers) > 0 {
		config.Resolvers = []network.ResolverSpecSpec{
			{
				DNSServers:  dnsServers,
				ConfigLayer: network.ConfigPlatform,
			},
		}
	}

	timeServers := formData.splitInputList(formData.timeServers)

	if len(timeServers) > 0 {
		config.TimeServers = []network.TimeServerSpecSpec{
			{
				NTPServers:  timeServers,
				ConfigLayer: network.ConfigPlatform,
			},
		}
	}

	ifaceSelected := formData.iface != "" && formData.iface != interfaceNone
	if ifaceSelected {
		config.Links = []network.LinkSpecSpec{
			{
				Name:        formData.iface,
				Logical:     false,
				Up:          true,
				Type:        nethelpers.LinkEther,
				ConfigLayer: network.ConfigPlatform,
			},
		}

		switch formData.mode {
		case modeDHCP:
			config.Operators = []network.OperatorSpecSpec{
				{
					Operator:  network.OperatorDHCP4,
					LinkName:  formData.iface,
					RequireUp: true,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: 1024,
					},
					ConfigLayer: network.ConfigPlatform,
				},
			}
		case modeStatic:
			config.Addresses, err = formData.buildAddresses(formData.iface)
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			if len(config.Addresses) == 0 {
				errs = multierror.Append(errs, fmt.Errorf("no addresses specified"))
			}

			config.Routes, err = formData.buildRoutes(formData.iface)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}

	if errs != nil {
		return nil, errs
	}

	return config, nil
}

func (formData *networkConfigFormData) parseAddresses(text string) ([]netip.Addr, error) {
	var errs error

	split := formData.splitInputList(text)
	addresses := make([]netip.Addr, 0, len(split))

	for _, address := range split {
		addr, err := netip.ParseAddr(address)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("address: %w", err))

			continue
		}

		addresses = append(addresses, addr)
	}

	if errs != nil {
		return nil, errs
	}

	return addresses, nil
}

func (formData *networkConfigFormData) buildAddresses(linkName string) ([]network.AddressSpecSpec, error) {
	var errs error

	addressesSplit := formData.splitInputList(formData.addresses)
	addresses := make([]network.AddressSpecSpec, 0, len(addressesSplit))

	for _, address := range addressesSplit {
		prefix, err := netip.ParsePrefix(address)
		if err != nil {
			errs = multierror.Append(errs, err)

			continue
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
			ConfigLayer: network.ConfigPlatform,
		})
	}

	if errs != nil {
		return nil, errs
	}

	return addresses, nil
}

func (formData *networkConfigFormData) buildRoutes(linkName string) ([]network.RouteSpecSpec, error) {
	gateway := strings.TrimSpace(formData.gateway)

	gatewayAddr, err := netip.ParseAddr(gateway)
	if err != nil {
		return nil, fmt.Errorf("gateway: %w", err)
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
			ConfigLayer: network.ConfigPlatform,
		},
	}, nil
}

func (formData *networkConfigFormData) splitInputList(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})

	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)

		if trimmed != "" {
			result = append(result, part)
		}
	}

	return result
}
