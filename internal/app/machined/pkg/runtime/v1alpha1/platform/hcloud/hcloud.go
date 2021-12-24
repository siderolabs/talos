// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/talos-systems/go-procfs/procfs"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

const (
	// HCloudExternalIPEndpoint is the local hcloud endpoint for the external IP.
	HCloudExternalIPEndpoint = "http://169.254.169.254/hetzner/v1/metadata/public-ipv4"

	// HCloudNetworkEndpoint is the local hcloud endpoint for the network-config.
	HCloudNetworkEndpoint = "http://169.254.169.254/hetzner/v1/metadata/network-config"

	// HCloudHostnameEndpoint is the local hcloud endpoint for the hostname.
	HCloudHostnameEndpoint = "http://169.254.169.254/hetzner/v1/metadata/hostname"

	// HCloudUserDataEndpoint is the local hcloud endpoint for the config.
	HCloudUserDataEndpoint = "http://169.254.169.254/hetzner/v1/userdata"
)

// NetworkConfig holds hcloud network-config info.
type NetworkConfig struct {
	Version int `yaml:"version"`
	Config  []struct {
		Mac        string `yaml:"mac_address"`
		Interfaces string `yaml:"name"`
		Subnets    []struct {
			NameServers []string `yaml:"dns_nameservers,omitempty"`
			Address     string   `yaml:"address,omitempty"`
			Gateway     string   `yaml:"gateway,omitempty"`
			Ipv4        bool     `yaml:"ipv4,omitempty"`
			Ipv6        bool     `yaml:"ipv6,omitempty"`
			Type        string   `yaml:"type"`
		} `yaml:"subnets"`
		Type string `yaml:"type"`
	} `yaml:"config"`
}

// Hcloud is the concrete type that implements the runtime.Platform interface.
type Hcloud struct{}

// Name implements the runtime.Platform interface.
func (h *Hcloud) Name() string {
	return "hcloud"
}

// ConfigurationNetwork implements the network configuration interface.
//nolint:gocyclo
func (h *Hcloud) ConfigurationNetwork(metadataNetworkConfig []byte, confProvider config.Provider) (config.Provider, error) {
	var unmarshalledNetworkConfig NetworkConfig

	if err := yaml.Unmarshal(metadataNetworkConfig, &unmarshalledNetworkConfig); err != nil {
		return nil, err
	}

	if unmarshalledNetworkConfig.Version != 1 {
		return nil, fmt.Errorf("network-config metadata version=%d is not supported", unmarshalledNetworkConfig.Version)
	}

	var machineConfig *v1alpha1.Config

	machineConfig, ok := confProvider.Raw().(*v1alpha1.Config)
	if !ok {
		return nil, fmt.Errorf("unable to determine machine config type")
	}

	if machineConfig.MachineConfig == nil {
		machineConfig.MachineConfig = &v1alpha1.MachineConfig{}
	}

	if machineConfig.MachineConfig.MachineNetwork == nil {
		machineConfig.MachineConfig.MachineNetwork = &v1alpha1.NetworkConfig{}
	}

	for _, network := range unmarshalledNetworkConfig.Config {
		if network.Type != "physical" {
			continue
		}

		iface := v1alpha1.Device{
			DeviceInterface: network.Interfaces,
			DeviceDHCP:      false,
		}

		for _, subnet := range network.Subnets {
			if subnet.Type == "dhcp" && subnet.Ipv4 {
				iface.DeviceDHCP = true
			}

			if subnet.Type == "static" {
				iface.DeviceAddresses = append(iface.DeviceAddresses,
					subnet.Address,
				)
			}

			if subnet.Gateway != "" && subnet.Ipv6 {
				iface.DeviceRoutes = []*v1alpha1.Route{
					{
						RouteNetwork: "::/0",
						RouteGateway: subnet.Gateway,
						RouteMetric:  1024,
					},
				}
			}
		}

		machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(
			machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces,
			&iface,
		)
	}

	return machineConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (h *Hcloud) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching hcloud network config from: %q", HCloudNetworkEndpoint)

	metadataNetworkConfig, err := download.Download(ctx, HCloudNetworkEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch network config from metadata service")
	}

	log.Printf("fetching machine config from: %q", HCloudUserDataEndpoint)

	machineConfigDl, err := download.Download(ctx, HCloudUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, err
	}

	confProvider, err := configloader.NewFromBytes(machineConfigDl)
	if err != nil {
		return nil, err
	}

	confProvider, err = h.ConfigurationNetwork(metadataNetworkConfig, confProvider)
	if err != nil {
		return nil, err
	}

	return confProvider.Bytes()
}

// Mode implements the runtime.Platform interface.
func (h *Hcloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (h *Hcloud) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching hostname from: %q", HCloudHostnameEndpoint)

	host, err := download.Download(ctx, HCloudHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		return nil, err
	}

	return host, nil
}

// ExternalIPs implements the runtime.Platform interface.
func (h *Hcloud) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	log.Printf("fetching externalIP from: %q", HCloudExternalIPEndpoint)

	exIP, err := download.Download(ctx, HCloudExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		return nil, err
	}

	if ip := net.ParseIP(string(exIP)); ip != nil {
		addrs = append(addrs, ip)
	}

	return addrs, nil
}

// KernelArgs implements the runtime.Platform interface.
func (h *Hcloud) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}
