// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package equinixmetal contains the Equinix Metal implementation of the [platform.Platform].
package equinixmetal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Event holds data to pass to the Equinix Metal event URL.
type Event struct {
	Type    string `json:"type"`
	Message string `json:"msg"`
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

// BGPNeighbor holds BGP neighbor info from the equinixmetal metadata.
type BGPNeighbor struct {
	AddressFamily int      `json:"address_family"`
	PeerIPs       []string `json:"peer_ips"`
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
func (p *EquinixMetal) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

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
func (p *EquinixMetal) ParseMetadata(ctx context.Context, equinixMetadata *MetadataConfig, st state.State) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	// 1. Links

	// translate the int returned from bond mode metadata to the type needed by network resources
	bondMode := nethelpers.BondMode(uint8(equinixMetadata.Network.Bonding.Mode))

	hostInterfaces, err := safe.StateListAll[*network.LinkStatus](ctx, st)
	if err != nil {
		return nil, fmt.Errorf("error listing host interfaces: %w", err)
	}

	bondSlaveIndexes := map[string]int{}
	firstBond := ""

	for _, iface := range equinixMetadata.Network.Interfaces {
		if iface.Bond == "" {
			continue
		}

		if firstBond == "" {
			firstBond = iface.Bond
		}

		found := false

		hostInterfaceIter := hostInterfaces.Iterator()

		for hostInterfaceIter.Next() {
			// match using permanent MAC address:
			// - bond interfaces don't have permanent addresses set, so we skip them this way
			// - if the bond is already configured, regular hardware address is overwritten with bond address
			if hostInterfaceIter.Value().TypedSpec().PermanentAddr.String() == iface.MAC {
				found = true

				slaveIndex := bondSlaveIndexes[iface.Bond]

				networkConfig.Links = append(networkConfig.Links,
					network.LinkSpecSpec{
						Name: hostInterfaceIter.Value().Metadata().ID(),
						Up:   true,
						BondSlave: network.BondSlave{
							MasterName: iface.Bond,
							SlaveIndex: slaveIndex,
						},
						ConfigLayer: network.ConfigPlatform,
					})

				bondSlaveIndexes[iface.Bond]++

				break
			}
		}

		if !found {
			log.Printf("interface with MAC %q wasn't found on the host, adding with the name from metadata", iface.MAC)

			slaveIndex := bondSlaveIndexes[iface.Bond]

			networkConfig.Links = append(networkConfig.Links,
				network.LinkSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Name:        iface.Name,
					Up:          true,
					BondSlave: network.BondSlave{
						MasterName: iface.Bond,
						SlaveIndex: slaveIndex,
					},
				})

			bondSlaveIndexes[iface.Bond]++
		}
	}

	for bondName := range bondSlaveIndexes {
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
	}

	// 2. addresses

	publicIPs := []string{}

	for _, addr := range equinixMetadata.Network.Addresses {
		if !(addr.Enabled && addr.Management) {
			continue
		}

		if addr.Public {
			publicIPs = append(publicIPs, addr.Address)
		}

		ipAddr, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", addr.Address, addr.CIDR))
		if err != nil {
			return nil, err
		}

		family := nethelpers.FamilyInet4
		if ipAddr.Addr().Is6() {
			family = nethelpers.FamilyInet6
		}

		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    firstBond,
				Address:     ipAddr,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      family,
			},
		)
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	// 3. routes
	var privateGateway netip.Addr

	for _, addr := range equinixMetadata.Network.Addresses {
		if !(addr.Enabled && addr.Management) {
			continue
		}

		ipAddr, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", addr.Address, addr.CIDR))
		if err != nil {
			return nil, err
		}

		family := nethelpers.FamilyInet4
		if ipAddr.Addr().Is6() {
			family = nethelpers.FamilyInet6
		}

		if addr.Public {
			// for "Public" address add the default route
			gw, err := netip.ParseAddr(addr.Gateway)
			if err != nil {
				return nil, err
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Gateway:     gw,
				OutLinkName: firstBond,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      family,
				Priority:    network.DefaultRouteMetric,
			}

			if addr.Family == 6 {
				route.Priority = 2 * network.DefaultRouteMetric
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		} else {
			// for "Private" addresses, we add a route that goes out the gateway for the private subnets.
			for _, privSubnet := range equinixMetadata.PrivateSubnets {
				gw, err := netip.ParseAddr(addr.Gateway)
				if err != nil {
					return nil, err
				}

				privateGateway = gw

				dest, err := netip.ParsePrefix(privSubnet)
				if err != nil {
					return nil, err
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					Destination: dest,
					OutLinkName: firstBond,
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

	// 5. platform metadata

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     p.Name(),
		Hostname:     equinixMetadata.Hostname,
		Region:       equinixMetadata.Metro,
		Zone:         equinixMetadata.Facility,
		InstanceType: equinixMetadata.Plan,
		InstanceID:   equinixMetadata.ID,
		ProviderID:   fmt.Sprintf("equinixmetal://%s", equinixMetadata.ID),
	}

	// 6. BGP neighbors

	for _, bgpNeighbor := range equinixMetadata.BGPNeighbors {
		if bgpNeighbor.AddressFamily != 4 {
			continue
		}

		for _, peerIP := range bgpNeighbor.PeerIPs {
			peer, err := netip.ParseAddr(peerIP)
			if err != nil {
				return nil, err
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Gateway:     privateGateway,
				Destination: netip.PrefixFrom(peer, 32),
				OutLinkName: firstBond,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      nethelpers.FamilyInet4,
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		}
	}

	return networkConfig, nil
}

// NetworkConfiguration implements the runtime.Platform interface.
func (p *EquinixMetal) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching equinix network config from: %q", EquinixMetalMetaDataEndpoint)

	metadataConfig, err := download.Download(ctx, EquinixMetalMetaDataEndpoint)
	if err != nil {
		return err
	}

	var meta MetadataConfig
	if err = json.Unmarshal(metadataConfig, &meta); err != nil {
		return err
	}

	networkConfig, err := p.ParseMetadata(ctx, &meta, st)
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
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, *eventURL, bytes.NewReader(eventData))
			if reqErr != nil {
				return reqErr
			}

			resp, reqErr := http.DefaultClient.Do(req)
			if resp != nil {
				io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024*1024)) //nolint:errcheck
				resp.Body.Close()                                           //nolint:errcheck
			}

			return retry.ExpectedError(reqErr)
		},
	)

	return err
}
