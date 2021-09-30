// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

const (
	// ScalewayMetadataEndpoint is the local Scaleway endpoint.
	ScalewayMetadataEndpoint = "http://169.254.42.42/conf?format=json"
)

// Scaleway is the concrete type that implements the runtime.Platform interface.
type Scaleway struct{}

// Name implements the runtime.Platform interface.
func (s *Scaleway) Name() string {
	return "scaleway"
}

// ConfigurationNetwork implements the network configuration interface.
func (s *Scaleway) ConfigurationNetwork(metadataConfig *instance.Metadata, confProvider config.Provider) (config.Provider, error) {
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

	iface := v1alpha1.Device{
		DeviceInterface: "eth0",
		DeviceDHCP:      true,
	}

	if metadataConfig.IPv6.Address != "" {
		iface.DeviceAddresses = append(iface.DeviceAddresses,
			fmt.Sprintf("%s/%s", metadataConfig.IPv6.Address, metadataConfig.IPv6.Netmask),
		)

		iface.DeviceRoutes = []*v1alpha1.Route{
			{
				RouteNetwork: "::/0",
				RouteGateway: metadataConfig.IPv6.Gateway,
				RouteMetric:  1024,
			},
		}
	}

	machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(
		machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces,
		&iface,
	)

	return confProvider, nil
}

// Configuration implements the runtime.Platform interface.
func (s *Scaleway) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching scaleway instance config from: %q ", ScalewayMetadataEndpoint)

	metadataDl, err := download.Download(ctx, ScalewayMetadataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, errors.ErrNoConfigSource
	}

	metadata := &instance.Metadata{}
	if err = json.Unmarshal(metadataDl, metadata); err != nil {
		return nil, errors.ErrNoConfigSource
	}

	log.Printf("fetching machine config from scaleway metadata server")

	instanceAPI := instance.NewMetadataAPI()

	machineConfigDl, err := instanceAPI.GetUserData("cloud-init")
	if err != nil {
		return nil, errors.ErrNoConfigSource
	}

	confProvider, err := configloader.NewFromBytes(machineConfigDl)
	if err != nil {
		return nil, err
	}

	confProvider, err = s.ConfigurationNetwork(metadata, confProvider)
	if err != nil {
		return nil, err
	}

	return confProvider.Bytes()
}

// Mode implements the runtime.Platform interface.
func (s *Scaleway) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (s *Scaleway) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching hostname from: %q", ScalewayMetadataEndpoint)

	metadataDl, err := download.Download(ctx, ScalewayMetadataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		return nil, err
	}

	metadata := &instance.Metadata{}
	if err = json.Unmarshal(metadataDl, metadata); err != nil {
		return nil, err
	}

	return []byte(metadata.Hostname), nil
}

// ExternalIPs implements the runtime.Platform interface.
func (s *Scaleway) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	log.Printf("fetching external IP from: %q", ScalewayMetadataEndpoint)

	metadataDl, err := download.Download(ctx, ScalewayMetadataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		return addrs, err
	}

	metadata := &instance.Metadata{}
	if err = json.Unmarshal(metadataDl, metadata); err != nil {
		return addrs, err
	}

	addrs = append(addrs, net.ParseIP(metadata.PublicIP.Address))

	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (s *Scaleway) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}
