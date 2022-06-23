// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/utils"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// Openstack is the concrete type that implements the runtime.Platform interface.
type Openstack struct{}

// Name implements the runtime.Platform interface.
func (o *Openstack) Name() string {
	return "openstack"
}

// ParseMetadata converts OpenStack metadata to platform network configuration.
//nolint:gocyclo,cyclop
func (o *Openstack) ParseMetadata(unmarshalledMetadataConfig *MetadataConfig, unmarshalledNetworkConfig *NetworkConfig, hostname string, extIPs []netaddr.IP) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if hostname == "" {
		hostname = unmarshalledMetadataConfig.Hostname
	}

	if hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	networkConfig.ExternalIPs = extIPs

	var dnsIPs []netaddr.IP

	for _, netsvc := range unmarshalledNetworkConfig.Services {
		if netsvc.Type == "dns" && netsvc.Address != "" {
			if ip, err := netaddr.ParseIP(netsvc.Address); err == nil {
				dnsIPs = append(dnsIPs, ip)
			} else {
				return nil, fmt.Errorf("failed to parse dns service ip: %w", err)
			}
		}
	}

	if len(dnsIPs) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	ifaces := make(map[string]string)

	for idx, netLinks := range unmarshalledNetworkConfig.Links {
		switch netLinks.Type {
		case "phy", "vif", "ovs":
			// We need to define name of interface by MAC
			// I hope it will solve after https://github.com/talos-systems/talos/issues/4203, https://github.com/talos-systems/talos/issues/3265
			ifaces[netLinks.ID] = fmt.Sprintf("eth%d", idx)

			networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
				Name:        ifaces[netLinks.ID],
				Up:          true,
				MTU:         uint32(netLinks.MTU),
				ConfigLayer: network.ConfigPlatform,
			})
		}
	}

	for _, ntwrk := range unmarshalledNetworkConfig.Networks {
		if ntwrk.ID == "" || ifaces[ntwrk.Link] == "" {
			continue
		}

		iface := ifaces[ntwrk.Link]

		switch ntwrk.Type {
		case "ipv4_dhcp":
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  iface,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: 1024,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		case "ipv6_dhcp", "ipv6_dhcpv6-stateless", "ipv6_dhcpv6-stateful":
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  iface,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: 1024,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		case "ipv4", "ipv6", "ipv6_slaac":
			// FIXME: we need to switch on/off slaac here
		default:
			log.Printf("network type %s is not supported", ntwrk.Type)

			continue
		}

		if ntwrk.Address != "" {
			ipPrefix, err := utils.IPPrefixFrom(ntwrk.Address, ntwrk.Netmask)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ip address: %w", err)
			}

			family := nethelpers.FamilyInet4
			if ipPrefix.IP().Is6() {
				family = nethelpers.FamilyInet6
			}

			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    iface,
					Address:     ipPrefix,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      family,
				},
			)

			if ntwrk.Gateway != "" {
				gw, err := netaddr.ParseIP(ntwrk.Gateway)
				if err != nil {
					return nil, fmt.Errorf("failed to parse gateway ip: %w", err)
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					OutLinkName: iface,
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      family,
					Priority:    1024,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)
			}
		}

		for _, route := range ntwrk.Routes {
			gw, err := netaddr.ParseIP(route.Gateway)
			if err != nil {
				return nil, fmt.Errorf("failed to parse route gateway: %w", err)
			}

			dest, err := utils.IPPrefixFrom(route.Network, route.Netmask)
			if err != nil {
				return nil, fmt.Errorf("failed to parse route network: %w", err)
			}

			family := nethelpers.FamilyInet4
			if dest.IP().Is6() {
				family = nethelpers.FamilyInet6
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Destination: dest,
				Gateway:     gw,
				OutLinkName: iface,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      family,
				Priority:    1024,
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		}
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (o *Openstack) Configuration(ctx context.Context, r state.State) (machineConfig []byte, err error) {
	_, _, machineConfig, err = o.configFromCD()
	if err != nil {
		_, _, machineConfig, err = o.configFromNetwork(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Some openstack setups does not allow you to change user-data,
	// so skip this case.
	if bytes.HasPrefix(machineConfig, []byte("#cloud-config")) {
		return nil, errors.ErrNoConfigSource
	}

	return machineConfig, nil
}

// Mode implements the runtime.Platform interface.
func (o *Openstack) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (o *Openstack) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (o *Openstack) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	metadataConfigDl, metadataNetworkConfigDl, _, err := o.configFromCD()
	if err != nil {
		metadataConfigDl, metadataNetworkConfigDl, _, err = o.configFromNetwork(ctx)
		if stderrors.Is(err, errors.ErrNoConfigSource) {
			err = nil
		}

		if err != nil {
			return err
		}
	}

	hostname := o.hostname(ctx)
	extIPs := o.externalIPs(ctx)

	var (
		unmarshalledMetadataConfig MetadataConfig
		unmarshalledNetworkConfig  NetworkConfig
	)

	// ignore errors unmarshaling, empty configs work just fine as empty default
	_ = json.Unmarshal(metadataConfigDl, &unmarshalledMetadataConfig)       //nolint:errcheck
	_ = json.Unmarshal(metadataNetworkConfigDl, &unmarshalledNetworkConfig) //nolint:errcheck

	networkConfig, err := o.ParseMetadata(&unmarshalledMetadataConfig, &unmarshalledNetworkConfig, string(hostname), extIPs)
	if err != nil {
		return err
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
