// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package alibabacloud contains the Alibabacloud implementation of the [platform.Platform].
package alibabacloud

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	platformerrors "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const alibabacloudInterfaceName = "eth0"

// Alibabacloud implements the platform interface for Alibaba Cloud.
type Alibabacloud struct{}

// Name returns the name of the platform.
func (a *Alibabacloud) Name() string {
	return "alibabacloud"
}

// Mode returns the platform mode.
func (a *Alibabacloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Configuration fetches machine configuration.
func (a *Alibabacloud) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from Alibaba Cloud")

	userdata, err := newMetadataClient().getUserData(ctx)
	if err != nil {
		return nil, err
	}

	if len(userdata) == 0 {
		return nil, platformerrors.ErrNoConfigSource
	}

	return userdata, nil
}

// KernelArgs implements the runtime.Platform interface.
func (a *Alibabacloud) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (a *Alibabacloud) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	metadata, err := a.getMetadata(ctx)
	if err != nil {
		return err
	}

	networkConfig, err := a.ParseMetadata(metadata)
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

// ParseMetadata converts Alibaba Cloud platform metadata into platform network config.
//
//nolint:gocyclo
func (a *Alibabacloud) ParseMetadata(metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
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

	if iface := metadata.PrimaryInterface; iface != nil {
		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        alibabacloudInterfaceName,
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		})

		hasIPv4 := len(iface.PrivateIPv4s) > 0 || metadata.PublicIPv4 != ""
		hasIPv6 := len(iface.IPv6s) > 0

		if !hasIPv4 && !hasIPv6 {
			hasIPv4 = true
		}

		if hasIPv4 {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  alibabacloudInterfaceName,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		}

		if hasIPv6 {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  alibabacloudInterfaceName,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		}
	}

	if len(metadata.DNSServers) > 0 {
		dnsIPs := make([]netip.Addr, 0, len(metadata.DNSServers))

		for _, dnsServer := range metadata.DNSServers {
			ip, err := netip.ParseAddr(dnsServer)
			if err != nil {
				return nil, fmt.Errorf("failed to parse DNS server %q: %w", dnsServer, err)
			}

			dnsIPs = append(dnsIPs, ip)
		}

		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	if len(metadata.NTPServers) > 0 {
		networkConfig.TimeServers = append(networkConfig.TimeServers, network.TimeServerSpecSpec{
			NTPServers:  metadata.NTPServers,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	if metadata.PublicIPv4 != "" {
		if ip, err := netip.ParseAddr(metadata.PublicIPv4); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     a.Name(),
		Hostname:     metadata.Hostname,
		Region:       metadata.Region,
		Zone:         metadata.Zone,
		InstanceType: metadata.InstanceType,
		InstanceID:   metadata.InstanceID,
		ProviderID:   fmt.Sprintf("%s.%s", metadata.Region, metadata.InstanceID),
		Tags:         metadata.Tags,
	}

	return networkConfig, nil
}
