// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package upcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

const (
	// UpCloudMetadataEndpoint is the local UpCloud endpoint.
	UpCloudMetadataEndpoint = "http://169.254.169.254/metadata/v1.json"

	// UpCloudExternalIPEndpoint is the local UpCloud endpoint for the external IP.
	UpCloudExternalIPEndpoint = "http://169.254.169.254/metadata/v1/network/interfaces/1/ip_addresses/1/address"

	// UpCloudHostnameEndpoint is the local UpCloud endpoint for the hostname.
	UpCloudHostnameEndpoint = "http://169.254.169.254/metadata/v1/hostname"

	// UpCloudUserDataEndpoint is the local UpCloud endpoint for the config.
	UpCloudUserDataEndpoint = "http://169.254.169.254/metadata/v1/user_data"
)

// MetaData represents a metadata Upcloud interface.
type MetaData struct {
	Hostname   string   `json:"hostname,omitempty"`
	InstanceID string   `json:"instance_id,omitempty"`
	PublicKeys []string `json:"public_keys,omitempty"`
	Region     string   `json:"region,omitempty"`

	Network struct {
		Interfaces []struct {
			Index       int `json:"index,omitempty"`
			IPAddresses []struct {
				Address  string   `json:"address,omitempty"`
				DHCP     bool     `json:"dhcp,omitempty"`
				DNS      []string `json:"dns,omitempty"`
				Family   string   `json:"family,omitempty"`
				Floating bool     `json:"floating,omitempty"`
				Gateway  string   `json:"gateway,omitempty"`
				Network  string   `json:"network,omitempty"`
			} `json:"ip_addresses,omitempty"`
			MAC         string `json:"mac,omitempty"`
			NetworkType string `json:"type,omitempty"`
			NetworkID   string `json:"network_id,omitempty"`
		} `json:"interfaces,omitempty"`
		DNS []string `json:"dns,omitempty"`
	} `json:"network,omitempty"`
}

// UpCloud is the concrete type that implements the runtime.Platform interface.
type UpCloud struct{}

// Name implements the runtime.Platform interface.
func (u *UpCloud) Name() string {
	return "upcloud"
}

// ConfigurationNetwork implements the network configuration interface.
//nolint:gocyclo
func (u *UpCloud) ConfigurationNetwork(metadataConfig []byte, confProvider config.Provider) (config.Provider, error) {
	var machineConfig *v1alpha1.Config

	machineConfig, ok := confProvider.Raw().(*v1alpha1.Config)
	if !ok {
		return nil, fmt.Errorf("unable to determine machine config type")
	}

	meta := &MetaData{}
	if err := json.Unmarshal(metadataConfig, meta); err != nil {
		return nil, err
	}

	if machineConfig.MachineConfig == nil {
		machineConfig.MachineConfig = &v1alpha1.MachineConfig{}
	}

	if machineConfig.MachineConfig.MachineNetwork == nil {
		machineConfig.MachineConfig.MachineNetwork = &v1alpha1.NetworkConfig{}
	}

	if machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces == nil {
		for _, addr := range meta.Network.Interfaces {
			if addr.Index <= 0 { // protect from negative interface name
				continue
			}

			iface := &v1alpha1.Device{
				DeviceInterface: fmt.Sprintf("eth%d", addr.Index-1),
			}

			for _, ip := range addr.IPAddresses {
				if ip.DHCP && ip.Family == "IPv4" {
					iface.DeviceDHCP = true
				}

				if !ip.DHCP {
					if ip.Floating {
						iface.DeviceAddresses = append(iface.DeviceAddresses, ip.Network)
					} else {
						iface.DeviceAddresses = append(iface.DeviceAddresses, ip.Address)

						if ip.Gateway != "" {
							iface.DeviceRoutes = append(iface.DeviceRoutes, &v1alpha1.Route{
								RouteNetwork: ip.Network,
								RouteGateway: ip.Gateway,
								RouteMetric:  1024,
							})
						}
					}
				}
			}

			machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces, iface)
		}
	}

	return machineConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (u *UpCloud) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching UpCloud instance config from: %q ", UpCloudMetadataEndpoint)

	metaConfigDl, err := download.Download(ctx, UpCloudMetadataEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch network config from metadata service")
	}

	log.Printf("fetching machine config from: %q", UpCloudUserDataEndpoint)

	machineConfigDl, err := download.Download(ctx, UpCloudUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, err
	}

	confProvider, err := configloader.NewFromBytes(machineConfigDl)
	if err != nil {
		return nil, err
	}

	confProvider, err = u.ConfigurationNetwork(metaConfigDl, confProvider)
	if err != nil {
		return nil, err
	}

	return confProvider.Bytes()
}

// Mode implements the runtime.Platform interface.
func (u *UpCloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (u *UpCloud) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching hostname from: %q", UpCloudHostnameEndpoint)

	host, err := download.Download(ctx, UpCloudHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		return nil, err
	}

	return host, nil
}

// ExternalIPs implements the runtime.Platform interface.
func (u *UpCloud) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	log.Printf("fetching external IP from: %q", UpCloudExternalIPEndpoint)

	exIP, err := download.Download(ctx, UpCloudExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		return addrs, err
	}

	addrs = append(addrs, net.ParseIP(string(exIP)))

	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (u *UpCloud) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{}
}
