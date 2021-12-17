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
	"net"

	"github.com/AlekSi/pointer"
	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
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

// ConfigurationNetwork implements the network configuration interface.
func (o *Oracle) ConfigurationNetwork(metadataNetworkConfig []byte, confProvider config.Provider) (config.Provider, error) {
	var machineConfig *v1alpha1.Config

	machineConfig, ok := confProvider.(*v1alpha1.Config)
	if !ok {
		return nil, fmt.Errorf("unable to determine machine config type")
	}

	if machineConfig.MachineConfig == nil {
		machineConfig.MachineConfig = &v1alpha1.MachineConfig{}
	}

	if machineConfig.MachineConfig.MachineNetwork == nil {
		machineConfig.MachineConfig.MachineNetwork = &v1alpha1.NetworkConfig{}
	}

	var interfaceAddresses []NetworkConfig

	if err := json.Unmarshal(metadataNetworkConfig, &interfaceAddresses); err != nil {
		return nil, err
	}

	if machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces == nil {
		for idx, iface := range interfaceAddresses {
			ipv6 := iface.Ipv6SubnetCidrBlock != "" && iface.Ipv6VirtualRouterIP != ""

			if ipv6 {
				device := &v1alpha1.Device{
					DeviceInterface:   fmt.Sprintf("eth%d", idx),
					DeviceDHCP:        true,
					DeviceDHCPOptions: &v1alpha1.DHCPOptions{DHCPIPv6: pointer.ToBool(true)},
				}

				machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces, device)
			}
		}
	}

	return confProvider, nil
}

// Configuration implements the platform.Platform interface.
func (o *Oracle) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching network config from %q", OracleNetworkEndpoint)

	metadataNetworkConfig, err := download.Download(ctx, OracleNetworkEndpoint,
		download.WithHeaders(map[string]string{"Authorization": "Bearer Oracle"}))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch network config from metadata service")
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

	confProvider, err := configloader.NewFromBytes(machineConfig)
	if err != nil {
		return nil, fmt.Errorf("error parsing machine config: %w", err)
	}

	confProvider, err = o.ConfigurationNetwork(metadataNetworkConfig, confProvider)
	if err != nil {
		return nil, err
	}

	return confProvider.Bytes()
}

// Hostname implements the platform.Platform interface.
func (o *Oracle) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching hostname from: %q", OracleHostnameEndpoint)

	hostname, err = download.Download(ctx, OracleHostnameEndpoint,
		download.WithHeaders(map[string]string{"Authorization": "Bearer Oracle"}),
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		return nil, err
	}

	return hostname, nil
}

// Mode implements the platform.Platform interface.
func (o *Oracle) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// ExternalIPs implements the runtime.Platform interface.
func (o *Oracle) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	return nil, nil
}

// KernelArgs implements the runtime.Platform interface.
func (o *Oracle) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}
