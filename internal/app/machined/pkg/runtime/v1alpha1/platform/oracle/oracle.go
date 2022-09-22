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
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	runtimeres "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

// NetworkConfig holds network interface meta config.
type NetworkConfig struct {
	HWAddr              string `json:"macAddr"`
	PrivateIP           string `json:"privateIp"`
	VirtualRouterIP     string `json:"virtualRouterIp"`
	SubnetCidrBlock     string `json:"subnetCidrBlock"`
	Ipv6SubnetCidrBlock string `json:"ipv6SubnetCidrBlock,omitempty"`
	Ipv6VirtualRouterIP string `json:"ipv6VirtualRouterIp,omitempty"`
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
		ipv6 := iface.Ipv6SubnetCidrBlock != "" && iface.Ipv6VirtualRouterIP != ""

		if ipv6 {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  fmt.Sprintf("eth%d", idx),
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: 1024,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		}
	}

	zone := metadata.AvailabilityDomain

	if idx := strings.LastIndex(zone, ":"); idx != -1 {
		zone = zone[:idx]
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     o.Name(),
		Hostname:     metadata.Hostname,
		Region:       strings.ToLower(metadata.Region),
		Zone:         strings.ToLower(zone),
		InstanceType: metadata.Shape,
		InstanceID:   metadata.ID,
		ProviderID:   fmt.Sprintf("oci://%s", metadata.ID),
	}

	return networkConfig, nil
}

// Configuration implements the platform.Platform interface.
func (o *Oracle) Configuration(ctx context.Context, r state.State) ([]byte, error) {
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
func (o *Oracle) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
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
