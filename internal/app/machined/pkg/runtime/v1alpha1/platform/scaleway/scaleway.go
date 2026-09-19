// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package scaleway provides the Scaleway platform implementation.
package scaleway

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"

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

func staticAddress(addr netip.Prefix) network.AddressSpecSpec {
	family := nethelpers.FamilyInet6
	if addr.Addr().Is4() {
		family = nethelpers.FamilyInet4
	}

	return network.AddressSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		LinkName:    "eth0",
		Address:     addr,
		Scope:       nethelpers.ScopeGlobal,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
		Family:      family,
	}
}

// ParseMetadata converts Scaleway platform metadata into platform network config.
//
//nolint:gocyclo
func (s *Scaleway) ParseMetadata(metadata *instance.Metadata) (*runtime.PlatformNetworkConfig, error) {
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

	networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		ConfigLayer: network.ConfigPlatform,
	})

	networkConfig.Routes = []network.RouteSpecSpec{
		staticRoute(nethelpers.FamilyInet4, metadataRoute, netip.Addr{}, 4*network.DefaultRouteMetric),
	}

	// Instances always run in routed IP mode: public IPs are attached to the instance as
	// host routes and configured statically. The `routed_ip_enabled` flag is deprecated in
	// the API and always true, so there is no legacy NAT mode left to detect.
	// See https://github.com/scaleway/scaleway-sdk-go/issues/3247.
	var defaultV4RouteSet bool

	for _, v4 := range metadata.PublicIpsV4 {
		addr, err := address.IPPrefixFrom(v4.Address, v4.Netmask)
		if err != nil {
			return nil, err
		}

		networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, addr.Addr())

		if v4.Gateway == "" {
			// older instances don't advertise a gateway, let DHCP configure the address and routes
			continue
		}

		networkConfig.Addresses = append(networkConfig.Addresses, staticAddress(addr))

		// Use the first advertised gateway so multiple public IPs don't create competing defaults.
		if defaultV4RouteSet {
			continue
		}

		gw, err := netip.ParseAddr(v4.Gateway)
		if err != nil {
			return nil, err
		}

		// /32 routed IPs require a host route to the gateway before the default route.
		networkConfig.Routes = append(networkConfig.Routes,
			staticRoute(nethelpers.FamilyInet4, netip.PrefixFrom(gw, gw.BitLen()), netip.Addr{}, 3*network.DefaultRouteMetric),
			staticRoute(nethelpers.FamilyInet4, netip.Prefix{}, gw, 2*network.DefaultRouteMetric),
		)

		defaultV4RouteSet = true
	}

	if len(metadata.PublicIpsV4) > 0 {
		networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
			Operator:  network.OperatorDHCP4,
			LinkName:  "eth0",
			RequireUp: true,
			DHCP4: network.DHCP4OperatorSpec{
				RouteMetric: network.DefaultRouteMetric,
				SkipRoutes:  defaultV4RouteSet,
			},
			ConfigLayer: network.ConfigPlatform,
		})
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

	var defaultV6RouteSet bool

	for _, v6 := range v6ips {
		addr, err := address.IPPrefixFrom(v6.Address, v6.Netmask)
		if err != nil {
			return nil, err
		}

		networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, addr.Addr())
		networkConfig.Addresses = append(networkConfig.Addresses, staticAddress(addr))

		// every address shares the same gateway, so the default route is only set once
		if v6.Gateway == "" || defaultV6RouteSet {
			continue
		}

		gw, err := netip.ParseAddr(v6.Gateway)
		if err != nil {
			return nil, err
		}

		networkConfig.Routes = append(networkConfig.Routes,
			staticRoute(nethelpers.FamilyInet6, netip.Prefix{}, gw, 2*network.DefaultRouteMetric),
		)

		defaultV6RouteSet = true
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

	log.Printf("fetching machine config from %q or %q", ScalewayUserDataEndpoint, ScalewayUserDataEndpointIPv6)

	return downloadAlternating(ctx, ScalewayUserDataEndpoint, ScalewayUserDataEndpointIPv6,
		download.WithLowSrcPort(),
		download.WithErrorOnNotFound(retry.ExpectedError(errors.ErrNoConfigSource)),
		download.WithErrorOnEmptyResponse(retry.ExpectedError(errors.ErrNoConfigSource)),
	)
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
func (s *Scaleway) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	// wait for devices to be ready before proceeding
	if err := netutils.WaitForDevicesReady(ctx, st); err != nil {
		return fmt.Errorf("error waiting for devices to be ready: %w", err)
	}

	log.Printf("fetching scaleway instance config from: %q or %q", ScalewayMetadataEndpoint, ScalewayMetadataEndpointIPv6)

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
