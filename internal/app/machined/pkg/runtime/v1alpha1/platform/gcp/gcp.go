// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package gcp contains the GCP implementation of the [platform.Platform].
package gcp

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strconv"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// GCP is the concrete type that implements the platform.Platform interface.
type GCP struct{}

// Name implements the platform.Platform interface.
func (g *GCP) Name() string {
	return "gcp"
}

// ParseMetadata converts GCP platform metadata into platform network config.
//
//nolint:gocyclo
func (g *GCP) ParseMetadata(metadata *MetadataConfig, interfaces []NetworkInterfaceConfig) (*runtime.PlatformNetworkConfig, error) {
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

	dns, _ := netip.ParseAddr(gcpResolverServer) //nolint:errcheck

	networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
		DNSServers:  []netip.Addr{dns},
		ConfigLayer: network.ConfigPlatform,
	})

	networkConfig.TimeServers = append(networkConfig.TimeServers, network.TimeServerSpecSpec{
		NTPServers:  []string{gcpTimeServer},
		ConfigLayer: network.ConfigPlatform,
	})

	region := metadata.Zone

	if idx := strings.LastIndex(region, "-"); idx != -1 {
		region = region[:idx]
	}

	for idx, iface := range interfaces {
		ifname := fmt.Sprintf("eth%d", idx)

		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        ifname,
			Up:          true,
			MTU:         uint32(iface.MTU),
			ConfigLayer: network.ConfigPlatform,
		})

		networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
			Operator: network.OperatorDHCP4,
			LinkName: ifname,
			DHCP4: network.DHCP4OperatorSpec{
				RouteMetric: network.DefaultRouteMetric,
			},
			RequireUp:   true,
			ConfigLayer: network.ConfigPlatform,
		})

		for _, ipv6addr := range iface.IPv6 {
			if ipv6addr == "" || iface.GatewayIPv6 == "" {
				continue
			}

			ipPrefix, err := netip.ParsePrefix(ipv6addr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ip address: %w", err)
			}

			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    ifname,
					Address:     ipPrefix,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      nethelpers.FamilyInet6,
				},
			)

			gw, err := netip.ParseAddr(iface.GatewayIPv6)
			if err != nil {
				return nil, err
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Gateway:     gw,
				OutLinkName: ifname,
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

	for _, iface := range interfaces {
		for _, ipStr := range iface.AccessConfigs {
			if ipStr.Type == "ONE_TO_ONE_NAT" {
				if ip, err := netip.ParseAddr(ipStr.ExternalIP); err == nil {
					networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
				}
			}
		}
	}

	preempted, _ := strconv.ParseBool(metadata.Preempted) //nolint:errcheck

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     g.Name(),
		Hostname:     metadata.Hostname,
		Region:       region,
		Zone:         metadata.Zone,
		InstanceType: metadata.InstanceType,
		InstanceID:   metadata.InstanceID,
		ProviderID:   fmt.Sprintf("gce://%s/%s/%s", metadata.ProjectID, metadata.Zone, metadata.Name),
		Spot:         preempted,
	}

	return networkConfig, nil
}

// Configuration implements the platform.Platform interface.
func (g *GCP) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from GCP metadata service")

	userdata, err := netutils.RetryFetch(ctx, g.fetchConfiguration)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(userdata) == "" {
		return nil, errors.ErrNoConfigSource
	}

	return []byte(userdata), nil
}

func (g *GCP) fetchConfiguration(ctx context.Context) (string, error) {
	userdata, err := metadata.InstanceAttributeValueWithContext(ctx, "user-data")
	if err != nil {
		if _, ok := err.(metadata.NotDefinedError); ok {
			return "", errors.ErrNoConfigSource
		}

		return "", retry.ExpectedError(err)
	}

	return userdata, nil
}

// Mode implements the platform.Platform interface.
func (g *GCP) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (g *GCP) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
		procfs.NewParameter(constants.KernelParamDashboardDisabled).Append("1"),
		// disable 'kexec' as GCP VMs sometimes are stuck on kexec, and normal soft reboot
		// doesn't take much longer on VMs
		procfs.NewParameter("sysctl.kernel.kexec_load_disabled").Append("1"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (g *GCP) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching gcp instance config")

	metadata, err := g.getMetadata(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive GCP metadata: %w", err)
	}

	network, err := g.getNetworkMetadata(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive GCP network metadata: %w", err)
	}

	networkConfig, err := g.ParseMetadata(metadata, network)
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
