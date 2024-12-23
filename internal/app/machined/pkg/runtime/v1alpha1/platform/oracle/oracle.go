// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package oracle provides the Oracle platform implementation.
package oracle

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"strings"

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

// NetworkConfig holds network interface meta config.
type NetworkConfig struct {
	HWAddr              string   `json:"macAddr"`
	PrivateIP           string   `json:"privateIp"`
	VirtualRouterIP     string   `json:"virtualRouterIp"`
	SubnetCidrBlock     string   `json:"subnetCidrBlock"`
	Ipv6SubnetCidrBlock string   `json:"ipv6SubnetCidrBlock,omitempty"`
	Ipv6VirtualRouterIP string   `json:"ipv6VirtualRouterIp,omitempty"`
	Ipv6Addresses       []string `json:"ipv6Addresses,omitempty"`
}

// Oracle is the concrete type that implements the platform.Platform interface.
type Oracle struct{}

// Name implements the platform.Platform interface.
func (o *Oracle) Name() string {
	return "oracle"
}

// ParseMetadata converts Oracle Cloud metadata into platform network configuration.
func (o *Oracle) ParseMetadata(interfaceAddresses []NetworkConfig, metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
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

	for idx, iface := range interfaceAddresses {
		ifname := fmt.Sprintf("eth%d", idx)

		if iface.Ipv6SubnetCidrBlock != "" && iface.Ipv6VirtualRouterIP != "" {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  ifname,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})

			gw, err := netip.ParseAddr(iface.Ipv6VirtualRouterIP)
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

	dns, _ := netip.ParseAddr(oracleResolverServer) //nolint:errcheck

	networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
		DNSServers:  []netip.Addr{dns},
		ConfigLayer: network.ConfigPlatform,
	})

	networkConfig.TimeServers = append(networkConfig.TimeServers, network.TimeServerSpecSpec{
		NTPServers:  []string{oracleTimeServer},
		ConfigLayer: network.ConfigPlatform,
	})

	zone := metadata.AvailabilityDomain

	if idx := strings.LastIndex(zone, ":"); idx != -1 {
		zone = zone[idx+1:]
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     o.Name(),
		Hostname:     metadata.Hostname,
		Region:       metadata.Region,
		Zone:         zone,
		InstanceType: metadata.Shape,
		InstanceID:   metadata.ID,
		ProviderID:   fmt.Sprintf("oci://%s", metadata.ID),
	}

	return networkConfig, nil
}

// Configuration implements the platform.Platform interface.
func (o *Oracle) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from: %q", OracleUserDataEndpoint)

	machineConfigDl, err := download.Download(ctx, OracleUserDataEndpoint,
		download.WithHeaders(map[string]string{"Authorization": "Bearer Oracle"}),
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, err
	}

	machineConfig, err := base64.StdEncoding.DecodeString(string(machineConfigDl))
	if err != nil {
		return nil, errors.ErrNoConfigSource
	}

	return machineConfig, nil
}

// Mode implements the platform.Platform interface.
func (o *Oracle) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (o *Oracle) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
		procfs.NewParameter(constants.KernelParamDashboardDisabled).Append("1"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (o *Oracle) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching oracle metadata from: %q", OracleMetadataEndpoint)

	metadata, err := o.getMetadata(ctx)
	if err != nil {
		return err
	}

	log.Printf("fetching network config from %q", OracleNetworkEndpoint)

	metadataNetworkConfigDl, err := download.Download(ctx, OracleNetworkEndpoint,
		download.WithHeaders(map[string]string{"Authorization": "Bearer Oracle"}))
	if err != nil {
		return fmt.Errorf("failed to fetch network config from: %w", err)
	}

	var interfaceAddresses []NetworkConfig

	if err = json.Unmarshal(metadataNetworkConfigDl, &interfaceAddresses); err != nil {
		return err
	}

	networkConfig, err := o.ParseMetadata(interfaceAddresses, metadata)
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
