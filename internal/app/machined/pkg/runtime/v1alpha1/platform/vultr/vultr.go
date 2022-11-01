// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vultr provides the Vultr platform implementation.
package vultr

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"net"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/vultr/metadata"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	runtimeres "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
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
func (v *Vultr) ParseMetadata(extIP []byte, metadata *metadata.MetaData) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if ip, err := netip.ParseAddr(string(extIP)); err == nil {
		networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
	}

	if metadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

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
			if addr.NetworkType != "private" {
				networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
					Operator:  network.OperatorDHCP4,
					LinkName:  iface,
					RequireUp: true,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: 1024,
					},
					ConfigLayer: network.ConfigPlatform,
				})
			} else {
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
			}
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
func (v *Vultr) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (v *Vultr) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching Vultr instance metadata from: %q", VultrMetadataEndpoint)

	metadata, err := v.getMetadata(ctx)
	if err != nil {
		return err
	}

	extIP, err := download.Download(ctx, VultrExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil && !stderrors.Is(err, errors.ErrNoExternalIPs) {
		return err
	}

	networkConfig, err := v.ParseMetadata(extIP, metadata)
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
