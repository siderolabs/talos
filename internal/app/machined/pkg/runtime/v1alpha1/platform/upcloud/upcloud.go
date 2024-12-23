// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package upcloud provides the UpCloud platform implementation.
package upcloud

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// UpCloud is the concrete type that implements the runtime.Platform interface.
type UpCloud struct{}

// Name implements the runtime.Platform interface.
func (u *UpCloud) Name() string {
	return "upcloud"
}

// ParseMetadata converts Upcloud metadata into platform network configuration.
//
//nolint:gocyclo
func (u *UpCloud) ParseMetadata(metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
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

	var (
		publicIPs []string
		dnsIPs    []netip.Addr
	)

	firstIP := true

	for _, addr := range metadata.Network.Interfaces {
		if addr.Index <= 0 { // protect from negative interface name
			continue
		}

		iface := fmt.Sprintf("eth%d", addr.Index-1)

		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        iface,
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		})

		for _, ip := range addr.IPAddresses {
			if firstIP {
				publicIPs = append(publicIPs, ip.Address)

				firstIP = false
			}

			for _, addr := range ip.DNS {
				if ipAddr, err := netip.ParseAddr(addr); err == nil {
					dnsIPs = append(dnsIPs, ipAddr)
				}
			}

			if ip.DHCP && ip.Family == "IPv4" {
				networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
					Operator:  network.OperatorDHCP4,
					LinkName:  iface,
					RequireUp: true,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: network.DefaultRouteMetric,
					},
					ConfigLayer: network.ConfigPlatform,
				})
			}

			if !ip.DHCP {
				ntwrk, err := netip.ParsePrefix(ip.Network)
				if err != nil {
					return nil, err
				}

				addr, err := netip.ParseAddr(ip.Address)
				if err != nil {
					return nil, err
				}

				ipPrefix := netip.PrefixFrom(addr, ntwrk.Bits())

				family := nethelpers.FamilyInet4

				if addr.Is6() {
					publicIPs = append(publicIPs, ip.Address)
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

				if ip.Gateway != "" {
					gw, err := netip.ParseAddr(ip.Gateway)
					if err != nil {
						return nil, err
					}

					route := network.RouteSpecSpec{
						ConfigLayer: network.ConfigPlatform,
						Gateway:     gw,
						Destination: ntwrk,
						OutLinkName: iface,
						Table:       nethelpers.TableMain,
						Protocol:    nethelpers.ProtocolStatic,
						Type:        nethelpers.TypeUnicast,
						Family:      family,
						Priority:    network.DefaultRouteMetric,
					}

					route.Normalize()

					networkConfig.Routes = append(networkConfig.Routes, route)
				}
			}
		}
	}

	if len(dnsIPs) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:   u.Name(),
		Hostname:   metadata.Hostname,
		Zone:       metadata.Zone,
		InstanceID: metadata.InstanceID,
		ProviderID: fmt.Sprintf("upcloud://%s", metadata.InstanceID),
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (u *UpCloud) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from: %q", UpCloudUserDataEndpoint)

	return download.Download(ctx, UpCloudUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the runtime.Platform interface.
func (u *UpCloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (u *UpCloud) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (u *UpCloud) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching UpCloud instance config from: %q", UpCloudMetadataEndpoint)

	metadata, err := u.getMetadata(ctx)
	if err != nil {
		return err
	}

	networkConfig, err := u.ParseMetadata(metadata)
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
