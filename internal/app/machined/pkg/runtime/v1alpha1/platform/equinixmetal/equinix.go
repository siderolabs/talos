// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package equinixmetal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	networkadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/network"
	networkctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// Event holds data to pass to the Equinix Metal event URL.
type Event struct {
	Type    string `json:"type"`
	Message string `json:"msg"`
}

// Metadata holds equinixmetal metadata info.
type Metadata struct {
	Hostname       string   `json:"hostname"`
	Network        Network  `json:"network"`
	PrivateSubnets []string `json:"private_subnets"`
}

// Network holds network info from the equinixmetal metadata.
type Network struct {
	Bonding    Bonding     `json:"bonding"`
	Interfaces []Interface `json:"interfaces"`
	Addresses  []Address   `json:"addresses"`
}

// Bonding holds bonding info from the equinixmetal metadata.
type Bonding struct {
	Mode int `json:"mode"`
}

// Interface holds interface info from the equinixmetal metadata.
type Interface struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	Bond string `json:"bond"`
}

// Address holds address info from the equinixmetal metadata.
type Address struct {
	Public     bool   `json:"public"`
	Management bool   `json:"management"`
	Enabled    bool   `json:"enabled"`
	CIDR       int    `json:"cidr"`
	Family     int    `json:"address_family"`
	Netmask    string `json:"netmask"`
	Network    string `json:"network"`
	Address    string `json:"address"`
	Gateway    string `json:"gateway"`
}

const (
	// EquinixMetalUserDataEndpoint is the local metadata endpoint for Equinix.
	EquinixMetalUserDataEndpoint = "https://metadata.platformequinix.com/userdata"
	// EquinixMetalMetaDataEndpoint is the local endpoint for machine info like networking.
	EquinixMetalMetaDataEndpoint = "https://metadata.platformequinix.com/metadata"
)

// EquinixMetal is a platform for EquinixMetal Metal cloud.
type EquinixMetal struct{}

// Name implements the platform.Platform interface.
func (p *EquinixMetal) Name() string {
	return "equinixMetal"
}

// Configuration implements the platform.Platform interface.
func (p *EquinixMetal) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", EquinixMetalUserDataEndpoint)

	return download.Download(ctx, EquinixMetalUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the platform.Platform interface.
func (p *EquinixMetal) Mode() runtime.Mode {
	return runtime.ModeMetal
}

// KernelArgs implements the runtime.Platform interface.
func (p *EquinixMetal) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS1,115200n8"),
	}
}

// ParseMetadata converts Equinix Metal metadata into Talos network configuration.
//
//nolint:gocyclo,cyclop
func (p *EquinixMetal) ParseMetadata(equinixMetadata *Metadata) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	// 1. Links

	// translate the int returned from bond mode metadata to the type needed by network resources
	bondMode := nethelpers.BondMode(uint8(equinixMetadata.Network.Bonding.Mode))

	// determine bond name and build list of interfaces enslaved by the bond
	bondName := ""

	hostInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error listing host interfaces: %w", err)
	}

	slaveIndex := 0

	for _, iface := range equinixMetadata.Network.Interfaces {
		if iface.Bond == "" {
			continue
		}

		if bondName != "" && iface.Bond != bondName {
			return nil, fmt.Errorf("encountered multiple bonds. this is unexpected in the equinix metal platform")
		}

		bondName = iface.Bond

		found := false

		for _, hostIf := range hostInterfaces {
			if hostIf.HardwareAddr.String() == iface.MAC {
				found = true

				networkConfig.Links = append(networkConfig.Links,
					network.LinkSpecSpec{
						Name: hostIf.Name,
						Up:   true,
						BondSlave: network.BondSlave{
							MasterName: bondName,
							SlaveIndex: slaveIndex,
						},
						ConfigLayer: network.ConfigPlatform,
					})
				slaveIndex++

				break
			}
		}

		if !found {
			log.Printf("interface with MAC %q wasn't found on the host, adding with the name from metadata", iface.MAC)

			networkConfig.Links = append(networkConfig.Links,
				network.LinkSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Name:        iface.Name,
					Up:          true,
					BondSlave: network.BondSlave{
						MasterName: bondName,
						SlaveIndex: slaveIndex,
					},
				})
			slaveIndex++
		}
	}

	bondLink := network.LinkSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		Name:        bondName,
		Logical:     true,
		Up:          true,
		Kind:        network.LinkKindBond,
		Type:        nethelpers.LinkEther,
		BondMaster: network.BondMasterSpec{
			Mode:       bondMode,
			DownDelay:  200,
			MIIMon:     100,
			UpDelay:    200,
			HashPolicy: nethelpers.BondXmitPolicyLayer34,
		},
	}

	networkadapter.BondMasterSpec(&bondLink.BondMaster).FillDefaults()

	networkConfig.Links = append(networkConfig.Links, bondLink)

	// 2. addresses

	for _, addr := range equinixMetadata.Network.Addresses {
		if !(addr.Enabled && addr.Management) {
			continue
		}

		ipAddr, err := netaddr.ParseIPPrefix(fmt.Sprintf("%s/%d", addr.Address, addr.CIDR))
		if err != nil {
			return nil, err
		}

		family := nethelpers.FamilyInet4
		if ipAddr.IP().Is6() {
			family = nethelpers.FamilyInet6
		}

		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    bondName,
				Address:     ipAddr,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      family,
			},
		)
	}

	// 3. routes

	for _, addr := range equinixMetadata.Network.Addresses {
		if !(addr.Enabled && addr.Management) {
			continue
		}

		ipAddr, err := netaddr.ParseIPPrefix(fmt.Sprintf("%s/%d", addr.Address, addr.CIDR))
		if err != nil {
			return nil, err
		}

		family := nethelpers.FamilyInet4
		if ipAddr.IP().Is6() {
			family = nethelpers.FamilyInet6
		}

		if addr.Public {
			// for "Public" address add the default route
			gw, err := netaddr.ParseIP(addr.Gateway)
			if err != nil {
				return nil, err
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Gateway:     gw,
				OutLinkName: bondName,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      family,
				Priority:    networkctrl.DefaultRouteMetric,
			}

			if addr.Family == 6 {
				route.Priority = 2 * networkctrl.DefaultRouteMetric
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		} else {
			// for "Private" addresses, we add a route that goes out the gateway for the private subnets.
			for _, privSubnet := range equinixMetadata.PrivateSubnets {
				gw, err := netaddr.ParseIP(addr.Gateway)
				if err != nil {
					return nil, err
				}

				dest, err := netaddr.ParseIPPrefix(privSubnet)
				if err != nil {
					return nil, err
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					Destination: dest,
					OutLinkName: bondName,
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      family,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)
			}
		}
	}

	// 4. hostname

	if equinixMetadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(equinixMetadata.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	return networkConfig, nil
}

// NetworkConfiguration implements the runtime.Platform interface.
func (p *EquinixMetal) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching equinix network config from: %q", EquinixMetalMetaDataEndpoint)

	metadataConfig, err := download.Download(ctx, EquinixMetalMetaDataEndpoint)
	if err != nil {
		return err
	}

	var equinixMetadata Metadata
	if err = json.Unmarshal(metadataConfig, &equinixMetadata); err != nil {
		return err
	}

	networkConfig, err := p.ParseMetadata(&equinixMetadata)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- networkConfig:
	}

	return nil
}

// FireEvent will take an event and pass it to an events server.
// nb: This is currently only used with Equinix Metal but we may find interesting ways
// to extend it for other event servers (Azure may have something similar?)
func (p *EquinixMetal) FireEvent(ctx context.Context, event Event) error {
	var eventURL *string
	if eventURL = procfs.ProcCmdline().Get(constants.KernelParamEquinixMetalEvents).First(); eventURL == nil {
		return errors.ErrNoEventURL
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = retry.Constant(5*time.Minute,
		retry.WithUnits(time.Second),
		retry.WithErrorLogging(true)).RetryWithContext(
		ctx,
		func(ctx context.Context) error {
			_, err = http.Post(*eventURL, "application/json", bytes.NewBuffer(eventData))

			return retry.ExpectedError(err)
		},
	)

	return err
}
