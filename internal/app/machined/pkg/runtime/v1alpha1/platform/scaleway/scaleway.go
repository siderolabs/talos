// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package scaleway provides the Scaleway platform implementation.
package scaleway

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"net/netip"
	"net/url"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/address"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Scaleway is the concrete type that implements the runtime.Platform interface.
type Scaleway struct{}

// Name implements the runtime.Platform interface.
func (s *Scaleway) Name() string {
	return "scaleway"
}

func staticRoute(family nethelpers.Family, dst netip.Prefix, gw netip.Addr, priority uint32) network.RouteSpecSpec {
	r := network.RouteSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		OutLinkName: "eth0",
		Destination: dst,
		Gateway:     gw,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Family:      family,
		Priority:    priority,
	}

	r.Normalize()

	return r
}

// ParseMetadata converts Scaleway platform metadata into platform network config.
//
//nolint:gocyclo,cyclop
func (s *Scaleway) ParseMetadata(metadata *Metadata) (*runtime.PlatformNetworkConfig, error) {
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

	networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		ConfigLayer: network.ConfigPlatform,
	})

	u, _ := url.Parse(ScalewayMetadataEndpoint)      //nolint:errcheck
	metadataAddr, _ := netip.ParseAddr(u.Hostname()) //nolint:errcheck
	networkConfig.Routes = []network.RouteSpecSpec{
		staticRoute(nethelpers.FamilyInet4, netip.PrefixFrom(metadataAddr, metadataAddr.BitLen()), netip.Addr{}, 4*network.DefaultRouteMetric),
	}

	if metadata.RoutedIPEnabled {
		for _, v4 := range metadata.PublicIpsV4 {
			publicIPs = append(publicIPs, v4.Address)

			addr, err := address.IPPrefixFrom(v4.Address, v4.Netmask)
			if err != nil {
				return nil, err
			}

			networkConfig.Addresses = append(networkConfig.Addresses, network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     addr,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      nethelpers.FamilyInet4,
			})

			if v4.Gateway != "" {
				gw, err := netip.ParseAddr(v4.Gateway)
				if err != nil {
					return nil, err
				}

				// /32 routed IPs require a host route to the gateway before the default route.
				networkConfig.Routes = append(networkConfig.Routes,
					staticRoute(nethelpers.FamilyInet4, netip.PrefixFrom(gw, gw.BitLen()), netip.Addr{}, 3*network.DefaultRouteMetric),
					staticRoute(nethelpers.FamilyInet4, netip.Prefix{}, gw, 2*network.DefaultRouteMetric),
				)
			}
		}
	} else {
		if metadata.PublicIP.Family == "inet" && metadata.PublicIP.Address != "" {
			publicIPs = append(publicIPs, metadata.PublicIP.Address)
		}

		if len(metadata.PublicIpsV4) > 0 {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  "eth0",
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		}
	}

	// IPv6: use PublicIpsV6 for all entries; fall back to the legacy IPv6 field on older instances.
	v6ips := metadata.PublicIpsV6
	if len(v6ips) == 0 && metadata.IPv6.Address != "" && metadata.IPv6.Netmask != "" && metadata.IPv6.Gateway != "" {
		v6ips = []instance.MetadataIP{{
			Address: metadata.IPv6.Address,
			Netmask: metadata.IPv6.Netmask,
			Gateway: metadata.IPv6.Gateway,
		}}
	}

	for _, v6 := range v6ips {
		addr, err := address.IPPrefixFrom(v6.Address, v6.Netmask)
		if err != nil {
			return nil, err
		}

		gw, err := netip.ParseAddr(v6.Gateway)
		if err != nil {
			return nil, err
		}

		publicIPs = append(publicIPs, v6.Address)
		networkConfig.Addresses = append(networkConfig.Addresses, network.AddressSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			LinkName:    "eth0",
			Address:     addr,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			Family:      nethelpers.FamilyInet6,
		})
		networkConfig.Routes = append(networkConfig.Routes,
			staticRoute(nethelpers.FamilyInet6, netip.Prefix{}, gw, 2*network.DefaultRouteMetric),
		)
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	zone, err := scw.ParseZone(metadata.Location.ZoneID)
	if err != nil {
		return nil, err
	}

	region, err := zone.Region()
	if err != nil {
		return nil, err
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     s.Name(),
		Hostname:     metadata.Hostname,
		Region:       region.String(),
		Zone:         zone.String(),
		InstanceType: metadata.CommercialType,
		InstanceID:   metadata.ID,
		ProviderID:   fmt.Sprintf("scaleway://instance/%s/%s", zone.String(), metadata.ID),
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (s *Scaleway) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from %q", ScalewayUserDataEndpoint)

	probeCtx, cancel := context.WithTimeout(ctx, metadataIPv4Timeout)
	cfg, err := download.Download(probeCtx, ScalewayUserDataEndpoint,
		download.WithLowSrcPort(),
		download.WithTimeout(metadataIPv4Timeout),
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))

	cancel()

	if err != nil && !stderrors.Is(err, errors.ErrNoConfigSource) {
		log.Printf("IPv4 user-data unreachable, falling back to IPv6 endpoint %q", ScalewayUserDataEndpointIPv6)

		cfg, err = download.Download(ctx, ScalewayUserDataEndpointIPv6,
			download.WithLowSrcPort(),
			download.WithErrorOnNotFound(errors.ErrNoConfigSource),
			download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	}

	return cfg, err
}

// Mode implements the runtime.Platform interface.
func (s *Scaleway) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (s *Scaleway) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
		procfs.NewParameter(constants.KernelParamDashboardDisabled).Append("1"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (s *Scaleway) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching scaleway instance config from: %q", ScalewayMetadataEndpoint)

	metadata, err := s.getMetadata(ctx)
	if err != nil {
		return err
	}

	networkConfig, err := s.ParseMetadata(metadata)
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
