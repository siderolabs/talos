// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vultr provides the Vultr platform implementation.
package vultr

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/vultr/metadata"

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

// Vultr is the concrete type that implements the runtime.Platform interface.
type Vultr struct{}

// Name implements the runtime.Platform interface.
func (v *Vultr) Name() string {
	return "vultr"
}

// ParseMetadata converts Vultr platform metadata into platform network config.
//
//nolint:gocyclo
func (v *Vultr) ParseMetadata(metadata *metadata.MetaData) (*runtime.PlatformNetworkConfig, error) {
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

	var publicIPs []string

	for i, addr := range metadata.Interfaces {
		iface := fmt.Sprintf("eth%d", i)

		link := network.LinkSpecSpec{
			Name:        iface,
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		}

		if addr.NetworkType == "private" {
			link.MTU = 1450
		}

		networkConfig.Links = append(networkConfig.Links, link)

		if addr.IPv4.Address != "" {
			if addr.NetworkType == "public" {
				publicIPs = append(publicIPs, addr.IPv4.Address)
			}

			ip, err := netip.ParseAddr(addr.IPv4.Address)
			if err != nil {
				return nil, err
			}

			netmask, err := netip.ParseAddr(addr.IPv4.Netmask)
			if err != nil {
				return nil, err
			}

			mask, _ := netmask.MarshalBinary() //nolint:errcheck // never fails
			ones, _ := net.IPMask(mask).Size()
			ipAddr := netip.PrefixFrom(ip, ones)

			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    iface,
					Address:     ipAddr,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      nethelpers.FamilyInet4,
				},
			)

			if addr.IPv4.Gateway != "" {
				gw, err := netip.ParseAddr(addr.IPv4.Gateway)
				if err != nil {
					return nil, err
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					OutLinkName: iface,
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)
			}
		} else {
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

		if addr.IPv6.Address != "" {
			if addr.NetworkType == "public" {
				publicIPs = append(publicIPs, addr.IPv6.Address)
			}
		}
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:   v.Name(),
		Hostname:   metadata.Hostname,
		Region:     metadata.Region.RegionCode,
		InstanceID: metadata.InstanceV2ID,
		ProviderID: fmt.Sprintf("vultr://%s", metadata.InstanceV2ID),
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
//
//nolint:stylecheck
func (v *Vultr) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from: %q", VultrUserDataEndpoint)

	return download.Download(ctx, VultrUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the runtime.Platform interface.
func (v *Vultr) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (v *Vultr) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (v *Vultr) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching Vultr instance metadata from: %q", VultrMetadataEndpoint)

	metadata, err := v.getMetadata(ctx)
	if err != nil {
		return err
	}

	networkConfig, err := v.ParseMetadata(metadata)
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
