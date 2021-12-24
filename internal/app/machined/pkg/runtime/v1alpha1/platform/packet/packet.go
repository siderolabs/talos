// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package packet

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// Metadata holds packet metadata info.
type Metadata struct {
	Hostname       string   `json:"hostname"`
	Network        Network  `json:"network"`
	PrivateSubnets []string `json:"private_subnets"`
}

// Network holds network info from the packet metadata.
type Network struct {
	Bonding    Bonding     `json:"bonding"`
	Interfaces []Interface `json:"interfaces"`
	Addresses  []Address   `json:"addresses"`
}

// Bonding holds bonding info from the packet metadata.
type Bonding struct {
	Mode int `json:"mode"`
}

// Interface holds interface info from the packet metadata.
type Interface struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	Bond string `json:"bond"`
}

// Address holds address info from the packet metadata.
type Address struct {
	Public  bool   `json:"public"`
	Enabled bool   `json:"enabled"`
	CIDR    int    `json:"cidr"`
	Family  int    `json:"address_family"`
	Netmask string `json:"netmask"`
	Network string `json:"network"`
	Address string `json:"address"`
	Gateway string `json:"gateway"`
}

const (
	// PacketUserDataEndpoint is the local metadata endpoint for Packet.
	PacketUserDataEndpoint = "https://metadata.platformequinix.com/userdata"
	// PacketMetaDataEndpoint is the local endpoint for machine info like networking.
	PacketMetaDataEndpoint = "https://metadata.platformequinix.com/metadata"
)

// Packet is a discoverer for non-cloud environments.
type Packet struct{}

// Name implements the platform.Platform interface.
func (p *Packet) Name() string {
	return "packet"
}

// Configuration implements the platform.Platform interface.
//nolint:gocyclo,cyclop
func (p *Packet) Configuration(ctx context.Context) ([]byte, error) {
	// Fetch and unmarshal both the talos machine config and the
	// metadata about the instance from packet's metadata server
	log.Printf("fetching machine config from: %q", PacketUserDataEndpoint)

	machineConfigDl, err := download.Download(ctx, PacketUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, err
	}

	log.Printf("fetching equinix network config from: %q", PacketMetaDataEndpoint)

	metadataConfig, err := download.Download(ctx, PacketMetaDataEndpoint)
	if err != nil {
		return nil, err
	}

	var unmarshalledMetadataConfig Metadata
	if err = json.Unmarshal(metadataConfig, &unmarshalledMetadataConfig); err != nil {
		return nil, err
	}

	confProvider, err := configloader.NewFromBytes(machineConfigDl)
	if err != nil {
		return nil, err
	}

	var machineConfig *v1alpha1.Config

	machineConfig, ok := confProvider.Raw().(*v1alpha1.Config)
	if !ok {
		return nil, fmt.Errorf("unable to determine machine config type")
	}

	// translate the int returned from bond mode metadata to the type needed by networkd
	bondMode := nethelpers.BondMode(uint8(unmarshalledMetadataConfig.Network.Bonding.Mode))

	// determine bond name and build list of interfaces enslaved by the bond
	devicesInBond := []string{}
	bondName := ""

	hostInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error listing host interfaces: %w", err)
	}

	for _, iface := range unmarshalledMetadataConfig.Network.Interfaces {
		if iface.Bond == "" {
			continue
		}

		if bondName != "" && iface.Bond != bondName {
			return nil, fmt.Errorf("encountered multiple bonds. this is unexpected in the equinix metal platform")
		}

		found := false

		for _, hostIf := range hostInterfaces {
			if hostIf.HardwareAddr.String() == iface.MAC {
				found = true

				devicesInBond = append(devicesInBond, hostIf.Name)

				break
			}
		}

		if !found {
			log.Printf("interface with MAC %q wasn't found on the host, skipping", iface.MAC)

			continue
		}

		bondName = iface.Bond
	}

	bondDev := v1alpha1.Device{
		DeviceInterface: bondName,
		DeviceDHCP:      false,
		DeviceBond: &v1alpha1.Bond{
			BondMode:       bondMode.String(),
			BondDownDelay:  200,
			BondMIIMon:     100,
			BondUpDelay:    200,
			BondHashPolicy: "layer3+4",
			BondInterfaces: devicesInBond,
		},
	}

	for _, addr := range unmarshalledMetadataConfig.Network.Addresses {
		bondDev.DeviceAddresses = append(bondDev.DeviceAddresses,
			fmt.Sprintf("%s/%d", addr.Address, addr.CIDR),
		)

		if addr.Public {
			// for "Public" address add the default route
			switch addr.Family {
			case 4:
				bondDev.DeviceRoutes = append(bondDev.DeviceRoutes, &v1alpha1.Route{
					RouteNetwork: "0.0.0.0/0",
					RouteGateway: addr.Gateway,
				})
			case 6:
				bondDev.DeviceRoutes = append(bondDev.DeviceRoutes, &v1alpha1.Route{
					RouteNetwork: "::/0",
					RouteGateway: addr.Gateway,
					RouteMetric:  2 * network.DefaultRouteMetric,
				})
			}
		} else {
			// for "Private" addresses, we add a route that goes out the gateway for the private subnets.
			for _, privSubnet := range unmarshalledMetadataConfig.PrivateSubnets {
				bondDev.DeviceRoutes = append(bondDev.DeviceRoutes, &v1alpha1.Route{
					RouteNetwork: privSubnet,
					RouteGateway: addr.Gateway,
				})
			}
		}
	}

	if machineConfig.MachineConfig == nil {
		machineConfig.MachineConfig = &v1alpha1.MachineConfig{}
	}

	if machineConfig.MachineConfig.MachineNetwork == nil {
		machineConfig.MachineConfig.MachineNetwork = &v1alpha1.NetworkConfig{}
	}

	machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(
		machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces,
		&bondDev,
	)

	return machineConfig.Bytes()
}

// Mode implements the platform.Platform interface.
func (p *Packet) Mode() runtime.Mode {
	return runtime.ModeMetal
}

// Hostname implements the platform.Platform interface.
func (p *Packet) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching equinix metadata from: %q", PacketMetaDataEndpoint)

	metadataConfig, err := download.Download(ctx, PacketMetaDataEndpoint)
	if err != nil {
		return nil, err
	}

	var unmarshalledMetadataConfig Metadata
	if err = json.Unmarshal(metadataConfig, &unmarshalledMetadataConfig); err != nil {
		return nil, err
	}

	return []byte(unmarshalledMetadataConfig.Hostname), nil
}

// ExternalIPs implements the runtime.Platform interface.
func (p *Packet) ExternalIPs(context.Context) (addrs []net.IP, err error) {
	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (p *Packet) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS1,115200n8"),
	}
}
