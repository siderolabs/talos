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
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

	var publicIPs []string

	if metadata.PublicIP.Address != "" {
		publicIPs = append(publicIPs, metadata.PublicIP.Address)
	}

	networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		ConfigLayer: network.ConfigPlatform,
	})

	gw, _ := netip.ParsePrefix("169.254.42.42/32") //nolint:errcheck
	route := network.RouteSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		OutLinkName: "eth0",
		Destination: gw,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Family:      nethelpers.FamilyInet4,
		Priority:    network.DefaultRouteMetric,
	}

	route.Normalize()
	networkConfig.Routes = []network.RouteSpecSpec{route}

	networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
		Operator:  network.OperatorDHCP4,
		LinkName:  "eth0",
		RequireUp: true,
		DHCP4: network.DHCP4OperatorSpec{
			RouteMetric: network.DefaultRouteMetric,
		},
		ConfigLayer: network.ConfigPlatform,
	})

	if metadata.IPv6.Address != "" || len(metadata.PublicIpsV6) > 0 {
		address := metadata.IPv6.Address
		netmask := metadata.IPv6.Netmask
		gateway := metadata.IPv6.Gateway

		if address == "" || netmask == "" || gateway == "" {
			address = metadata.PublicIpsV6[0].Address
			netmask = metadata.PublicIpsV6[0].Netmask
			gateway = metadata.PublicIpsV6[0].Gateway
		}

		bits, err := strconv.Atoi(netmask)
		if err != nil {
			return nil, err
		}

		ip, err := netip.ParseAddr(address)
		if err != nil {
			return nil, err
		}

		addr := netip.PrefixFrom(ip, bits)

		publicIPs = append(publicIPs, address)
		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     addr,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      nethelpers.FamilyInet6,
			},
		)

		gw, err := netip.ParseAddr(gateway)
		if err != nil {
			return nil, err
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

	zoneParts := strings.Split(metadata.Location.ZoneID, "-")
	if len(zoneParts) > 2 {
		zoneParts = zoneParts[:2]
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     s.Name(),
		Hostname:     metadata.Hostname,
		Region:       strings.Join(zoneParts, "-"),
		Zone:         metadata.Location.ZoneID,
		InstanceType: metadata.CommercialType,
		InstanceID:   metadata.ID,
		ProviderID:   fmt.Sprintf("scaleway://instance/%s/%s", metadata.Location.ZoneID, metadata.ID),
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
//
//nolint:stylecheck
func (s *Scaleway) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from %q", ScalewayUserDataEndpoint)

	return download.Download(ctx, ScalewayUserDataEndpoint,
		download.WithLowSrcPort(),
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the runtime.Platform interface.
func (s *Scaleway) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (s *Scaleway) KernelArgs(string) procfs.Parameters {
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
