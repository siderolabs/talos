// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway

import (
	"context"
	"encoding/json"
	"log"
	"strconv"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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

// ParseMetadata converts Scaleway met.
func (s *Scaleway) ParseMetadata(metadataConfig *instance.Metadata) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if metadataConfig.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadataConfig.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	if metadataConfig.PublicIP.Address != "" {
		ip, err := netaddr.ParseIP(metadataConfig.PublicIP.Address)
		if err != nil {
			return nil, err
		}

		networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
	}

	networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		ConfigLayer: network.ConfigPlatform,
	})

	networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
		Operator:  network.OperatorDHCP4,
		LinkName:  "eth0",
		RequireUp: true,
		DHCP4: network.DHCP4OperatorSpec{
			RouteMetric: 1024,
		},
		ConfigLayer: network.ConfigPlatform,
	})

	if metadataConfig.IPv6.Address != "" {
		bits, err := strconv.Atoi(metadataConfig.IPv6.Netmask)
		if err != nil {
			return nil, err
		}

		ip, err := netaddr.ParseIP(metadataConfig.IPv6.Address)
		if err != nil {
			return nil, err
		}

		addr := netaddr.IPPrefixFrom(ip, uint8(bits))

		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     addr,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      nethelpers.FamilyInet6,
			},
		)

		gw, err := netaddr.ParseIP(metadataConfig.IPv6.Gateway)
		if err != nil {
			return nil, err
		}

		route := network.RouteSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Gateway:     gw,
			OutLinkName: "eth0",
			Table:       nethelpers.TableMain,
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Family:      nethelpers.FamilyInet6,
			Priority:    1024,
		}

		route.Normalize()

		networkConfig.Routes = append(networkConfig.Routes, route)
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (s *Scaleway) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from scaleway metadata server")

	instanceAPI := instance.NewMetadataAPI()

	machineConfigDl, err := instanceAPI.GetUserData("cloud-init")
	if err != nil {
		return nil, errors.ErrNoConfigSource
	}

	return machineConfigDl, nil
}

// Mode implements the runtime.Platform interface.
func (s *Scaleway) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (s *Scaleway) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (s *Scaleway) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching scaleway instance config from: %q ", ScalewayMetadataEndpoint)

	metadataDl, err := download.Download(ctx, ScalewayMetadataEndpoint)
	if err != nil {
		return err
	}

	metadata := &instance.Metadata{}
	if err = json.Unmarshal(metadataDl, metadata); err != nil {
		return err
	}

	return nil
}
