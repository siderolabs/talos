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
	"slices"
	"sync/atomic"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
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

// Scaleway is the concrete type that implements the runtime.Platform interface.
type Scaleway struct {
	lastSuccessfulIPv6 atomic.Bool
}

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

func staticIPv6Address(addr netip.Prefix) network.AddressSpecSpec {
	return network.AddressSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		LinkName:    "eth0",
		Address:     addr,
		Scope:       nethelpers.ScopeGlobal,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
		Family:      nethelpers.FamilyInet6,
	}
}

func appendExternalIP(networkConfig *runtime.PlatformNetworkConfig, addr netip.Addr) {
	if slices.Contains(networkConfig.ExternalIPs, addr) {
		return
	}

	networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, addr)
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

	// Keep IPv4 addressing and routing under DHCP, for both routed and legacy
	// NAT instances. Public metadata addresses are only used for ExternalIPs;
	// neither a gateway nor provisioning_mode reliably distinguishes NAT mode.
	needsDHCP4 := len(metadata.PublicIpsV4) > 0 || metadata.PublicIP.Family == "inet"

	for _, v4 := range metadata.PublicIpsV4 {
		addr, err := netip.ParseAddr(v4.Address)
		if err != nil || !addr.Is4() {
			log.Printf("skipping malformed Scaleway public IPv4 address %q", v4.Address)

			continue
		}

		appendExternalIP(networkConfig, addr)
	}

	// Older metadata only exposed public_ip. Preserve it as an external address
	// even when the modern list is absent or contains malformed entries.
	if metadata.PublicIP.Family == "inet" {
		legacyV4, err := netip.ParseAddr(metadata.PublicIP.Address)
		if err != nil || !legacyV4.Is4() {
			log.Printf("skipping malformed Scaleway legacy public IPv4 address %q", metadata.PublicIP.Address)
		} else {
			appendExternalIP(networkConfig, legacyV4)
		}
	}

	// DHCP can recover IPv4 configuration even when advertised addresses are malformed.
	if needsDHCP4 {
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

	ipv6Addresses := parseIPv6MetadataIPs(metadata.PublicIpsV6)

	// Fall back to the legacy IPv6 object when no modern entry was usable.
	if len(ipv6Addresses) == 0 && metadata.IPv6.Address != "" {
		ipv6Addresses = parseIPv6MetadataIPs([]instance.MetadataIP{{
			Address: metadata.IPv6.Address,
			Netmask: metadata.IPv6.Netmask,
			Gateway: metadata.IPv6.Gateway,
		}})
	}

	var defaultIPv6Gateway netip.Addr

	for _, ip := range ipv6Addresses {
		appendExternalIP(networkConfig, ip.address.Addr())
		networkConfig.Addresses = append(networkConfig.Addresses, staticIPv6Address(ip.address))

		if !defaultIPv6Gateway.IsValid() {
			defaultIPv6Gateway = ip.gateway
		}
	}

	// Emit one default route using the first usable gateway in metadata order.
	if defaultIPv6Gateway.IsValid() {
		networkConfig.Routes = append(networkConfig.Routes,
			staticRoute(nethelpers.FamilyInet6, netip.Prefix{}, defaultIPv6Gateway, 2*network.DefaultRouteMetric),
		)
	}

	if len(ipv6Addresses) > 0 {
		// Scaleway uses SLAAC, not DHCPv6. The kernel accepts RA routes and
		// MTU updates, but Talos does not consume RA DNS options. Reachable
		// IPv6 resolvers also allow the default time.cloudflare.com NTP server.
		// Include them on dual-stack nodes too: IPv4 metadata does not establish
		// IPv4 connectivity. Resolver merging preserves DHCP's IPv4 DNS servers.
		resolverSpec := network.ResolverSpecSpec{
			NameServers: []network.NameServerSpec{
				{Addr: netip.MustParseAddr("2606:4700:4700::1111")},
				{Addr: netip.MustParseAddr("2001:4860:4860::8888")},
			},
			ConfigLayer: network.ConfigPlatform,
		}
		resolverSpec.Convert()

		networkConfig.Resolvers = append(networkConfig.Resolvers, resolverSpec)
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

	return s.downloadAlternating(ctx, ScalewayUserDataEndpoint, ScalewayUserDataEndpointIPv6,
		download.WithLowSrcPort(),
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource),
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
