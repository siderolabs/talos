// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oracle

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// Ref: https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/gettingmetadata.htm
const (
	// OracleHostnameEndpoint is the local metadata endpoint for the hostname.
	OracleHostnameEndpoint = "http://169.254.169.254/opc/v2/instance/hostname"
	// OracleUserDataEndpoint is the local metadata endpoint inside of Oracle Cloud.
	OracleUserDataEndpoint = "http://169.254.169.254/opc/v2/instance/metadata/user_data"
	// OracleNetworkEndpoint is the local network metadata endpoint inside of Oracle Cloud.
	OracleNetworkEndpoint = "http://169.254.169.254/opc/v2/vnics/"
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
func (o *Oracle) ParseMetadata(interfaceAddresses []NetworkConfig, hostname string) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(hostname); err != nil {
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

	return networkConfig, nil
}

// Configuration implements the platform.Platform interface.
func (o *Oracle) Configuration(ctx context.Context) ([]byte, error) {
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
func (o *Oracle) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching network config from %q", OracleNetworkEndpoint)

	metadataNetworkConfig, err := download.Download(ctx, OracleNetworkEndpoint,
		download.WithHeaders(map[string]string{"Authorization": "Bearer Oracle"}))
	if err != nil {
		return fmt.Errorf("failed to fetch network config from metadata service: %w", err)
	}

	var interfaceAddresses []NetworkConfig

	if err = json.Unmarshal(metadataNetworkConfig, &interfaceAddresses); err != nil {
		return err
	}

	log.Printf("fetching hostname from: %q", OracleHostnameEndpoint)

	hostname, _ := download.Download(ctx, OracleHostnameEndpoint, //nolint:errcheck
		download.WithHeaders(map[string]string{"Authorization": "Bearer Oracle"}),
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))

	networkConfig, err := o.ParseMetadata(interfaceAddresses, string(hostname))
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
