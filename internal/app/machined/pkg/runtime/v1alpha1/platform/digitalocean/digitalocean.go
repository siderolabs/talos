// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package digitalocean contains the Digital Ocean implementation of the [platform.Platform].
package digitalocean

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strconv"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/address"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// DigitalOcean is the concrete type that implements the platform.Platform interface.
type DigitalOcean struct{}

// Name implements the platform.Platform interface.
func (d *DigitalOcean) Name() string {
	return "digital-ocean"
}

// ParseMetadata converts DigitalOcean platform metadata into platform network config.
//
//nolint:gocyclo,cyclop
func (d *DigitalOcean) ParseMetadata(metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if metadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	if len(metadata.DNS.Nameservers) > 0 {
		var dnsIPs []netip.Addr

		for _, dnsIP := range metadata.DNS.Nameservers {
			if ip, err := netip.ParseAddr(dnsIP); err == nil {
				dnsIPs = append(dnsIPs, ip)
			}
		}

		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		ConfigLayer: network.ConfigPlatform,
	})

	var publicIPs []string

	for _, iface := range metadata.Interfaces["public"] {
		if iface.IPv4 != nil {
			ifAddr, err := address.IPPrefixFrom(iface.IPv4.IPAddress, iface.IPv4.Netmask)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ip address: %w", err)
			}

			publicIPs = append(publicIPs, iface.IPv4.IPAddress)

			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    "eth0",
					Address:     ifAddr,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      nethelpers.FamilyInet4,
				},
			)

			if iface.IPv4.Gateway != "" {
				gw, err := netip.ParseAddr(iface.IPv4.Gateway)
				if err != nil {
					return nil, fmt.Errorf("failed to parse gateway ip: %w", err)
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    network.DefaultRouteMetric,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)

				metaServer, _ := netip.ParsePrefix("169.254.169.254/32") //nolint:errcheck

				networkConfig.Routes = append(networkConfig.Routes, network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					OutLinkName: "eth0",
					Destination: metaServer,
					Gateway:     gw,
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    512,
				})
			}
		}

		if iface.IPv6 != nil {
			ifAddr, err := address.IPPrefixFrom(iface.IPv6.IPAddress, strconv.Itoa(iface.IPv6.CIDR))
			if err != nil {
				return nil, fmt.Errorf("failed to parse ip address: %w", err)
			}

			publicIPs = append(publicIPs, iface.IPv6.IPAddress)
			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    "eth0",
					Address:     ifAddr,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      nethelpers.FamilyInet6,
				},
			)

			if iface.IPv6.Gateway != "" {
				gw, err := netip.ParseAddr(iface.IPv6.Gateway)
				if err != nil {
					return nil, fmt.Errorf("failed to parse gateway ip: %w", err)
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet6,
					Priority:    2 * network.DefaultRouteMetric,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)
			}
		}

		if iface.AnchorIPv4 != nil {
			ifAddr, err := address.IPPrefixFrom(iface.AnchorIPv4.IPAddress, iface.AnchorIPv4.Netmask)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ip address: %w", err)
			}

			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    "eth0",
					Address:     ifAddr,
					Scope:       nethelpers.ScopeLink,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      nethelpers.FamilyInet4,
				},
			)
		}
	}

	for idx, iface := range metadata.Interfaces["private"] {
		ifName := fmt.Sprintf("eth%d", idx+1)

		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        ifName,
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		})

		if iface.IPv4 != nil {
			ifAddr, err := address.IPPrefixFrom(iface.IPv4.IPAddress, iface.IPv4.Netmask)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ip address: %w", err)
			}

			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    ifName,
					Address:     ifAddr,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      nethelpers.FamilyInet4,
				},
			)
		}
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:   d.Name(),
		Hostname:   metadata.Hostname,
		Region:     metadata.Region,
		InstanceID: strconv.Itoa(metadata.DropletID),
		ProviderID: fmt.Sprintf("digitalocean://%d", metadata.DropletID),
	}

	return networkConfig, nil
}

// Configuration implements the platform.Platform interface.
func (d *DigitalOcean) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from: %q", DigitalOceanUserDataEndpoint)

	return download.Download(ctx, DigitalOceanUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the platform.Platform interface.
func (d *DigitalOcean) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (d *DigitalOcean) KernelArgs(string) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0").Append("tty0").Append("tty1"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (d *DigitalOcean) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching DigitalOcean instance config from: %q ", DigitalOceanMetadataEndpoint)

	metadata, err := d.getMetadata(ctx)
	if err != nil {
		return err
	}

	networkConfig, err := d.ParseMetadata(metadata)
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
