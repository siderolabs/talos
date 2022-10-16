// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"
	"net/netip"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/utils"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"

	networkadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/network"
)

// Openstack is the concrete type that implements the runtime.Platform interface.
type Openstack struct{}

// Name implements the runtime.Platform interface.
func (o *Openstack) Name() string {
	return "openstack"
}

func PrettyPrint(v interface{}) (err error) {
	b, err := json.Marshal(v)
	if err == nil {
		fmt.Printf(string(b))
	}
	return
}

// ParseMetadata converts OpenStack metadata to platform network configuration.
//
//nolint:gocyclo,cyclop
func (o *Openstack) ParseMetadata(ctx context.Context, unmarshalledMetadataConfig *MetadataConfig, unmarshalledNetworkConfig *NetworkConfig, hostname string, extIPs []netip.Addr, st state.State) (*runtime.PlatformNetworkConfig, error) {
	fmt.Printf("Parsing metadata...")

	networkConfig := &runtime.PlatformNetworkConfig{}

	if hostname == "" {
		hostname = unmarshalledMetadataConfig.Hostname
	}

	if hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	networkConfig.ExternalIPs = extIPs

	var dnsIPs []netip.Addr

	for _, netsvc := range unmarshalledNetworkConfig.Services {
		if netsvc.Type == "dns" && netsvc.Address != "" {
			if ip, err := netip.ParseAddr(netsvc.Address); err == nil {
				dnsIPs = append(dnsIPs, ip)
			} else {
				return nil, fmt.Errorf("failed to parse dns service ip: %w", err)
			}
		}
	}

	if len(dnsIPs) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	hostInterfaces, err := safe.StateList[*network.LinkStatus](ctx, st, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
	if err != nil {
		return nil, fmt.Errorf("error listing host interfaces: %w", err)
	}

	ifaces := make(map[string]string)
	bondLinks := make(map[string]string)

	// Bonds

	bondIndex := 0

	for _, netLink := range unmarshalledNetworkConfig.Links {
		switch netLink.Type {
		case "bond":
			// Bond master
			mode, err := nethelpers.BondModeByName(netLink.BondMode)

			if err != nil {
				return nil, fmt.Errorf("invalid bond_mode: %w", err)
			}

			hashPolicy, err := nethelpers.BondXmitHashPolicyByName(netLink.BondHashPolicy)

			if err != nil {
				return nil, fmt.Errorf("invalid bond_xmit_hash_policy: %w", err)
			}

			bondName := fmt.Sprintf("bond%d", bondIndex)

			bondLink := network.LinkSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Name:        bondName,
				Logical:     true,
				Up:          true,
				MTU:         uint32(netLink.MTU),
				Kind:        network.LinkKindBond,
				Type:        nethelpers.LinkEther,
				BondMaster: network.BondMasterSpec{
					Mode:       mode,
					MIIMon:     netLink.BondMIIMon,
					HashPolicy: hashPolicy,
					UpDelay:    200,
					DownDelay:  200,
					LACPRate:   nethelpers.LACPRateFast,
				},
			}

			networkadapter.BondMasterSpec(&bondLink.BondMaster).FillDefaults()
			networkConfig.Links = append(networkConfig.Links, bondLink)

			for _, bondLink := range netLink.BondLinks {
				bondLinks[bondLink] = bondName
			}

			bondIndex++
		}
	}

	bondSlaveIndexes := make(map[string]int)

	// Interfaces

	for _, netLink := range unmarshalledNetworkConfig.Links {
		switch netLink.Type {
		case "phy", "vif", "ovs":
			hostInterfaceIter := safe.IteratorFromList(hostInterfaces)

			for hostInterfaceIter.Next() {
				if strings.EqualFold(hostInterfaceIter.Value().TypedSpec().PermanentAddr.String(), netLink.Mac) {
					ifaces[netLink.ID] = hostInterfaceIter.Value().Metadata().ID()

					link := network.LinkSpecSpec{
						Name:        ifaces[netLink.ID],
						Up:          true,
						MTU:         uint32(netLink.MTU),
						ConfigLayer: network.ConfigPlatform,
					}

					bondName := bondLinks[netLink.ID]

					if bondName != "" {
						link.BondSlave = network.BondSlave{
							MasterName: bondName,
							SlaveIndex: bondSlaveIndexes[bondName],
						}

						bondSlaveIndexes[bondName]++
					}

					networkConfig.Links = append(networkConfig.Links, link)

					break
				}
			}
		}
	}

	for _, ntwrk := range unmarshalledNetworkConfig.Networks {
		if ntwrk.ID == "" || ifaces[ntwrk.Link] == "" {
			continue
		}

		iface := ifaces[ntwrk.Link]

		switch ntwrk.Type {
		case "ipv4_dhcp":
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  iface,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric:         1024,
					SkipHostnameRequest: true,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		case "ipv6_dhcp", "ipv6_dhcpv6-stateless", "ipv6_dhcpv6-stateful":
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  iface,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric:         1024,
					SkipHostnameRequest: true,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		case "ipv4", "ipv6", "ipv6_slaac":
			// FIXME: we need to switch on/off slaac here
		default:
			log.Printf("network type %s is not supported", ntwrk.Type)

			continue
		}

		if ntwrk.Address != "" {
			ipPrefix, err := utils.IPPrefixFrom(ntwrk.Address, ntwrk.Netmask)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ip address: %w", err)
			}

			family := nethelpers.FamilyInet4
			if ipPrefix.Addr().Is6() {
				family = nethelpers.FamilyInet6
			}

			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    iface,
					Address:     ipPrefix,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      family,
				},
			)

			if ntwrk.Gateway != "" {
				gw, err := netip.ParseAddr(ntwrk.Gateway)
				if err != nil {
					return nil, fmt.Errorf("failed to parse gateway ip: %w", err)
				}

				route := network.RouteSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					Gateway:     gw,
					OutLinkName: iface,
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      family,
					Priority:    1024,
				}

				route.Normalize()

				networkConfig.Routes = append(networkConfig.Routes, route)
			}
		}

		for _, route := range ntwrk.Routes {
			gw, err := netip.ParseAddr(route.Gateway)
			if err != nil {
				return nil, fmt.Errorf("failed to parse route gateway: %w", err)
			}

			dest, err := utils.IPPrefixFrom(route.Network, route.Netmask)
			if err != nil {
				return nil, fmt.Errorf("failed to parse route network: %w", err)
			}

			family := nethelpers.FamilyInet4
			if dest.Addr().Is6() {
				family = nethelpers.FamilyInet6
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Destination: dest,
				Gateway:     gw,
				OutLinkName: iface,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      family,
				Priority:    1024,
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		}
	}

	PrettyPrint(networkConfig.Links)

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (o *Openstack) Configuration(ctx context.Context, r state.State) (machineConfig []byte, err error) {
	_, _, machineConfig, err = o.configFromCD()
	if err != nil {
		_, _, machineConfig, err = o.configFromNetwork(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Some openstack setups does not allow you to change user-data,
	// so skip this case.
	if bytes.HasPrefix(machineConfig, []byte("#cloud-config")) {
		return nil, errors.ErrNoConfigSource
	}

	return machineConfig, nil
}

// Mode implements the runtime.Platform interface.
func (o *Openstack) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (o *Openstack) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (o *Openstack) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	fmt.Printf("NetworkConfiguration...")
	metadataConfigDl, metadataNetworkConfigDl, _, err := o.configFromCD()
	if err != nil {
		fmt.Printf("NetworkConfiguration... 2")
		metadataConfigDl, metadataNetworkConfigDl, _, err = o.configFromNetwork(ctx)
		if stderrors.Is(err, errors.ErrNoConfigSource) {
			fmt.Printf("NetworkConfiguration... 3")
			err = nil
		}

		if err != nil {
			fmt.Printf("NetworkConfiguration... 4")
			return err
		}
	}

	fmt.Printf("NetworkConfiguration... 5")

	hostname := o.hostname(ctx)
	extIPs := o.externalIPs(ctx)

	var (
		unmarshalledMetadataConfig MetadataConfig
		unmarshalledNetworkConfig  NetworkConfig
	)

	// ignore errors unmarshaling, empty configs work just fine as empty default
	_ = json.Unmarshal(metadataConfigDl, &unmarshalledMetadataConfig)       //nolint:errcheck
	_ = json.Unmarshal(metadataNetworkConfigDl, &unmarshalledNetworkConfig) //nolint:errcheck

	networkConfig, err := o.ParseMetadata(ctx, &unmarshalledMetadataConfig, &unmarshalledNetworkConfig, string(hostname), extIPs, st)
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
