// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vultr

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/talos-systems/go-procfs/procfs"
	"github.com/vultr/metadata"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

const (
	// VultrMetadataEndpoint is the local Vultr endpoint fot the instance metadata.
	VultrMetadataEndpoint = "http://169.254.169.254/v1.json"
	// VultrExternalIPEndpoint is the local Vultr endpoint for the external IP.
	VultrExternalIPEndpoint = "http://169.254.169.254/latest/meta-data/public-ipv4"
	// VultrHostnameEndpoint is the local Vultr endpoint for the hostname.
	VultrHostnameEndpoint = "http://169.254.169.254/latest/meta-data/hostname"
	// VultrUserDataEndpoint is the local Vultr endpoint for the config.
	VultrUserDataEndpoint = "http://169.254.169.254/latest/user-data"
)

// Vultr is the concrete type that implements the runtime.Platform interface.
type Vultr struct{}

// Name implements the runtime.Platform interface.
func (v *Vultr) Name() string {
	return "vultr"
}

// ConfigurationNetwork implements the network configuration interface.
func (v *Vultr) ConfigurationNetwork(metadataConfig []byte, confProvider config.Provider) (config.Provider, error) {
	var machineConfig *v1alpha1.Config

	machineConfig, ok := confProvider.(*v1alpha1.Config)
	if !ok {
		return nil, fmt.Errorf("unable to determine machine config type")
	}

	meta := &metadata.MetaData{}
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
		for i, addr := range meta.Interfaces {
			iface := &v1alpha1.Device{
				DeviceInterface: fmt.Sprintf("eth%d", i),
			}

			if addr.IPv4.Address != "" {
				iface.DeviceDHCP = true
			}

			if addr.NetworkType == "private" {
				iface.DeviceMTU = 1450

				if addr.IPv4.Address != "" {
					mask, _ := net.IPMask(net.ParseIP(addr.IPv4.Netmask).To4()).Size()

					iface.DeviceDHCP = false
					iface.DeviceAddresses = append(iface.DeviceAddresses,
						fmt.Sprintf("%s/%d", addr.IPv4.Address, mask),
					)
				}
			}

			machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces, iface)
		}
	}

	return confProvider, nil
}

// Configuration implements the runtime.Platform interface.
func (v *Vultr) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching Vultr instance config from: %q ", VultrMetadataEndpoint)

	metaConfigDl, err := download.Download(ctx, VultrMetadataEndpoint)
	if err != nil {
		return nil, errors.ErrNoConfigSource
	}

	log.Printf("fetching machine config from: %q", VultrUserDataEndpoint)

	machineConfigDl, err := download.Download(ctx, VultrUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, err
	}

	confProvider, err := configloader.NewFromBytes(machineConfigDl)
	if err != nil {
		return nil, err
	}

	confProvider, err = v.ConfigurationNetwork(metaConfigDl, confProvider)
	if err != nil {
		return nil, err
	}

	return confProvider.Bytes()
}

// Mode implements the runtime.Platform interface.
func (v *Vultr) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (v *Vultr) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching hostname from: %q", VultrHostnameEndpoint)

	hostname, err = download.Download(ctx, VultrHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		return nil, err
	}

	return hostname, nil
}

// ExternalIPs implements the runtime.Platform interface.
func (v *Vultr) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	log.Printf("fetching external IP from: %q", VultrExternalIPEndpoint)

	exIP, err := download.Download(ctx, VultrExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		return nil, err
	}

	if addr := net.ParseIP(string(exIP)); addr != nil {
		addrs = append(addrs, addr)
	}

	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (v *Vultr) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{}
}
