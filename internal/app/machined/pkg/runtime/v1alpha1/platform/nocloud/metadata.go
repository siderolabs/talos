// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nocloud provides the NoCloud platform implementation.
package nocloud

import (
	"bufio"
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-blockdevice/blockdevice/filesystem"
	"github.com/siderolabs/go-blockdevice/blockdevice/probe"
	"golang.org/x/sys/unix"
	yaml "gopkg.in/yaml.v3"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/internal/pkg/smbios"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const (
	configISOLabel          = "cidata"
	configNetworkConfigPath = "network-config"
	configMetaDataPath      = "meta-data"
	configUserDataPath      = "user-data"
	mnt                     = "/mnt"
)

// NetworkCloudInitConfig wraps nocloud network config to match cloud-init format.
type NetworkCloudInitConfig struct {
	Config NetworkConfig `yaml:"network"`
}

// NetworkConfig holds network-config info.
type NetworkConfig struct {
	Version   int                 `yaml:"version"`
	Config    []ConfigV1          `yaml:"config,omitempty"`
	Ethernets map[string]Ethernet `yaml:"ethernets,omitempty"`
	Bonds     map[string]Bond     `yaml:"bonds,omitempty"`
	VLANs     map[string]VLAN     `yaml:"vlans,omitempty"`
}

// ConfigV1 holds nocloud v1 config.
type ConfigV1 struct {
	Mac        string `yaml:"mac_address,omitempty"`
	Interfaces string `yaml:"name,omitempty"`
	MTU        uint32 `yaml:"mtu,omitempty"`
	Subnets    []struct {
		Address string `yaml:"address,omitempty"`
		Netmask string `yaml:"netmask,omitempty"`
		Gateway string `yaml:"gateway,omitempty"`
		Type    string `yaml:"type"`
	} `yaml:"subnets,omitempty"`
	Address        []string      `yaml:"address,omitempty"`
	Type           string        `yaml:"type"`
	BondInterfaces []string      `yaml:"bond_interfaces,omitempty"`
	VlanID         uint16        `yaml:"vlan_id,omitempty"`
	VlanLink       string        `yaml:"vlan_link,omitempty"`
	Params         NetworkParams `yaml:"params,omitempty"`
}

// Ethernet holds network interface info.
type Ethernet struct {
	Match struct {
		Name   string `yaml:"name,omitempty"`
		HWAddr string `yaml:"macaddress,omitempty"`
	} `yaml:"match,omitempty"`
	DHCPv4      bool     `yaml:"dhcp4,omitempty"`
	DHCPv6      bool     `yaml:"dhcp6,omitempty"`
	Address     []string `yaml:"addresses,omitempty"`
	Gateway4    string   `yaml:"gateway4,omitempty"`
	Gateway6    string   `yaml:"gateway6,omitempty"`
	MTU         uint32   `yaml:"mtu,omitempty"`
	NameServers struct {
		Search  []string `yaml:"search,omitempty"`
		Address []string `yaml:"addresses,omitempty"`
	} `yaml:"nameservers,omitempty"`
	Routes []struct {
		To     string `yaml:"to,omitempty"`
		Via    string `yaml:"via,omitempty"`
		Metric uint32 `yaml:"metric,omitempty"`
		Table  uint32 `yaml:"table,omitempty"`
		OnLink bool   `yaml:"on-link,omitempty"`
	} `yaml:"routes,omitempty"`
	RoutingPolicy []struct { // TODO
		From  string `yaml:"froom,omitempty"`
		Table uint32 `yaml:"table,omitempty"`
	} `yaml:"routing-policy,omitempty"`
}

// Bond holds bonding interface info.
type Bond struct {
	Ethernet   `yaml:",inline"`
	Interfaces []string `yaml:"interfaces,omitempty"`
	Params     struct {
		Mode       string `yaml:"mode,omitempty"`
		LACPRate   string `yaml:"lacp-rate,omitempty"`
		HashPolicy string `yaml:"transmit-hash-policy,omitempty"`
		MIIMon     uint32 `yaml:"mii-monitor-interval,omitempty"`
		UpDelay    uint32 `yaml:"up-delay,omitempty"`
		DownDelay  uint32 `yaml:"down-delay,omitempty"`
	} `yaml:"parameters,omitempty"`
}

// VLAN holds vlan interface info.
type VLAN struct {
	Ethernet `yaml:",inline"`
	ID       uint16 `yaml:"id,omitempty"`
	Link     string `yaml:"link,omitempty"`
}

// MetadataConfig holds meta info.
type MetadataConfig struct {
	Hostname     string `yaml:"hostname,omitempty"`
	InternalDNS  string `yaml:"local-hostname,omitempty"`
	ExternalDNS  string `yaml:"public-hostname,omitempty"`
	InstanceID   string `yaml:"instance-id,omitempty"`
	InstanceType string `yaml:"instance-type,omitempty"`
	ProviderID   string `yaml:"provider-id,omitempty"`
	Region       string `yaml:"region,omitempty"`
	Zone         string `yaml:"zone,omitempty"`
}

// NetworkParams holds network parameters (mostly bond for v1 network-config).
type NetworkParams struct {
	BondLACPRate       string `yaml:"bond-lacp-rate,omitempty"`
	BondMiimon         uint32 `yaml:"bond-miimon,omitempty"`
	BondMode           string `yaml:"bond-mode,omitempty"`
	BondXmitHashPolicy string `yaml:"bond-xmit-hash-policy,omitempty"`
	UpDelay            uint32 `yaml:"up-delay,omitempty"`
	DownDelay          uint32 `yaml:"down-delay,omitempty"`
}

func (n *Nocloud) configFromNetwork(ctx context.Context, metaBaseURL string, r state.State) (metaConfig []byte, networkConfig []byte, machineConfig []byte, err error) {
	log.Printf("fetching meta config from: %q", metaBaseURL+configMetaDataPath)

	if err = netutils.Wait(ctx, r); err != nil {
		return nil, nil, nil, err
	}

	metaConfig, err = download.Download(ctx, metaBaseURL+configMetaDataPath)
	if err != nil {
		metaConfig = nil
	}

	log.Printf("fetching network config from: %q", metaBaseURL+configNetworkConfigPath)

	networkConfig, err = download.Download(ctx, metaBaseURL+configNetworkConfigPath)
	if err != nil {
		networkConfig = nil
	}

	log.Printf("fetching machine config from: %q", metaBaseURL+configUserDataPath)

	machineConfig, err = download.Download(ctx, metaBaseURL+configUserDataPath,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))

	return metaConfig, networkConfig, machineConfig, err
}

//nolint:gocyclo
func (n *Nocloud) configFromCD(ctx context.Context, r state.State) (metaConfig []byte, networkConfig []byte, machineConfig []byte, err error) {
	if err := netutils.WaitForDevicesReady(ctx, r); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to wait for devices: %w", err)
	}

	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(strings.ToLower(configISOLabel))
	if err != nil {
		dev, err = probe.GetDevWithFileSystemLabel(strings.ToUpper(configISOLabel))
		if err != nil {
			return nil, nil, nil, errors.ErrNoConfigSource
		}
	}

	//nolint:errcheck
	defer dev.Close()

	sb, err := filesystem.Probe(dev.Path)
	if err != nil || sb == nil {
		return nil, nil, nil, errors.ErrNoConfigSource
	}

	log.Printf("found config disk (cidata) at %s", dev.Path)

	if err = unix.Mount(dev.Path, mnt, sb.Type(), unix.MS_RDONLY, ""); err != nil {
		return nil, nil, nil, errors.ErrNoConfigSource
	}

	log.Printf("fetching meta config from: cidata/%s", configMetaDataPath)

	metaConfig, err = os.ReadFile(filepath.Join(mnt, configMetaDataPath))
	if err != nil {
		log.Printf("failed to read %s: %s", configMetaDataPath, err)

		metaConfig = nil
	}

	log.Printf("fetching network config from: cidata/%s", configNetworkConfigPath)

	networkConfig, err = os.ReadFile(filepath.Join(mnt, configNetworkConfigPath))
	if err != nil {
		log.Printf("failed to read %s: %s", configNetworkConfigPath, err)

		networkConfig = nil
	}

	log.Printf("fetching machine config from: cidata/%s", configUserDataPath)

	machineConfig, err = os.ReadFile(filepath.Join(mnt, configUserDataPath))
	if err != nil {
		log.Printf("failed to read %s: %s", configUserDataPath, err)

		machineConfig = nil
	}

	if err = unix.Unmount(mnt, 0); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to unmount: %w", err)
	}

	if machineConfig == nil {
		err = errors.ErrNoConfigSource
	}

	return metaConfig, networkConfig, machineConfig, err
}

//nolint:gocyclo
func (n *Nocloud) acquireConfig(ctx context.Context, r state.State) (metadataNetworkConfigDl, machineConfigDl []byte, metadata *MetadataConfig, err error) {
	s, err := smbios.GetSMBIOSInfo()
	if err != nil {
		return nil, nil, nil, err
	}

	var (
		metaBaseURL, hostname, instanceID string
		networkSource                     bool
	)

	options := strings.Split(s.SystemInformation.SerialNumber, ";")
	for _, option := range options {
		parts := strings.SplitN(option, "=", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "ds":
				if parts[1] == "nocloud-net" {
					networkSource = true
				}
			case "s":
				var u *url.URL

				u, err = url.Parse(parts[1])
				if err == nil && strings.HasPrefix(u.Scheme, "http") {
					if strings.HasSuffix(u.Path, "/") {
						metaBaseURL = parts[1]
					} else {
						metaBaseURL = parts[1] + "/"
					}
				}
			case "h":
				hostname = parts[1]

			case "i":
				instanceID = parts[1]
			}
		}
	}

	var metadataConfigDl []byte

	if networkSource && metaBaseURL != "" {
		metadataConfigDl, metadataNetworkConfigDl, machineConfigDl, err = n.configFromNetwork(ctx, metaBaseURL, r)
	} else {
		metadataConfigDl, metadataNetworkConfigDl, machineConfigDl, err = n.configFromCD(ctx, r)
	}

	metadata = &MetadataConfig{}

	if metadataConfigDl != nil {
		_ = yaml.Unmarshal(metadataConfigDl, metadata) //nolint:errcheck
	}

	if hostname != "" {
		metadata.Hostname = hostname
	}

	if instanceID != "" {
		metadata.InstanceID = instanceID
	}

	// Some providers may provide the hostname via user-data instead of meta-data (e.g. Proxmox VE)
	// As long as the user doesn't use it for machine config, it can still be used to obtain the hostname
	if metadata.Hostname == "" && metadata.InternalDNS == "" && machineConfigDl != nil {
		fallbackMetadata := &MetadataConfig{}
		_ = yaml.Unmarshal(machineConfigDl, fallbackMetadata) //nolint:errcheck
		metadata.Hostname = fallbackMetadata.Hostname
		metadata.InternalDNS = fallbackMetadata.InternalDNS
	}

	return metadataNetworkConfigDl, machineConfigDl, metadata, err
}

//nolint:gocyclo,cyclop
func (n *Nocloud) applyNetworkConfigV1(ctx context.Context, config *NetworkConfig, st state.State, networkConfig *runtime.PlatformNetworkConfig) (bool, error) {
	hostInterfaces, err := safe.StateListAll[*network.LinkStatus](ctx, st)
	if err != nil {
		return false, fmt.Errorf("error listing host interfaces: %w", err)
	}

	var needsReconcile bool

	parseSubnets := func(ntwrk ConfigV1, name string) error {
		for _, subnet := range ntwrk.Subnets {
			switch subnet.Type {
			case "dhcp", "dhcp4":
				networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
					Operator:  network.OperatorDHCP4,
					LinkName:  name,
					RequireUp: true,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: network.DefaultRouteMetric,
					},
					ConfigLayer: network.ConfigPlatform,
				})
			case "static", "static6":
				family := nethelpers.FamilyInet4

				if subnet.Type == "static6" {
					family = nethelpers.FamilyInet6
				}

				ipPrefix, err := netip.ParsePrefix(subnet.Address)
				if err != nil {
					ip, err := netip.ParseAddr(subnet.Address)
					if err != nil {
						return err
					}

					netmask, err := netip.ParseAddr(subnet.Netmask)
					if err != nil {
						return err
					}

					mask, _ := netmask.MarshalBinary() //nolint:errcheck // never fails
					ones, _ := net.IPMask(mask).Size()
					ipPrefix = netip.PrefixFrom(ip, ones)
				}

				networkConfig.Addresses = append(networkConfig.Addresses,
					network.AddressSpecSpec{
						ConfigLayer: network.ConfigPlatform,
						LinkName:    name,
						Address:     ipPrefix,
						Scope:       nethelpers.ScopeGlobal,
						Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
						Family:      family,
					},
				)

				if subnet.Gateway != "" {
					gw, err := netip.ParseAddr(subnet.Gateway)
					if err != nil {
						return err
					}

					route := network.RouteSpecSpec{
						ConfigLayer: network.ConfigPlatform,
						Gateway:     gw,
						OutLinkName: name,
						Table:       nethelpers.TableMain,
						Protocol:    nethelpers.ProtocolStatic,
						Type:        nethelpers.TypeUnicast,
						Family:      family,
						Priority:    network.DefaultRouteMetric,
					}

					if family == nethelpers.FamilyInet6 {
						route.Priority = 2 * network.DefaultRouteMetric
					}

					route.Normalize()

					networkConfig.Routes = append(networkConfig.Routes, route)
				}
			case "ipv6_dhcpv6-stateful":
				networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
					Operator:  network.OperatorDHCP6,
					LinkName:  name,
					RequireUp: true,
					DHCP6: network.DHCP6OperatorSpec{
						RouteMetric: 2 * network.DefaultRouteMetric,
					},
					ConfigLayer: network.ConfigPlatform,
				})
			}
		}

		return nil
	}

	physicalNameMap := map[string]string{}

	type enslavedLink struct {
		bondName   string
		slaveIndex int
	}

	enslavedLinks := map[string]enslavedLink{}

	for _, ntwrk := range config.Config {
		switch ntwrk.Type {
		case "nameserver":
			dnsIPs := make([]netip.Addr, 0, len(ntwrk.Address))

			for i := range ntwrk.Address {
				if ip, err := netip.ParseAddr(ntwrk.Address[i]); err == nil {
					dnsIPs = append(dnsIPs, ip)
				} else {
					return false, err
				}
			}

			networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
				DNSServers:  dnsIPs,
				ConfigLayer: network.ConfigPlatform,
			})
		case "bond":
			name := ntwrk.Interfaces

			mode, err := nethelpers.BondModeByName(ntwrk.Params.BondMode)
			if err != nil {
				return false, fmt.Errorf("invalid mode: %w", err)
			}

			hashPolicy, err := nethelpers.BondXmitHashPolicyByName(ntwrk.Params.BondXmitHashPolicy)
			if err != nil {
				return false, fmt.Errorf("invalid transmit-hash-policy: %w", err)
			}

			lacpRate, err := nethelpers.LACPRateByName(ntwrk.Params.BondLACPRate)
			if err != nil {
				return false, fmt.Errorf("invalid lacp-rate: %w", err)
			}

			bondLink := network.LinkSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Name:        name,
				Logical:     true,
				Up:          true,
				Kind:        network.LinkKindBond,
				Type:        nethelpers.LinkEther,
				BondMaster: network.BondMasterSpec{
					Mode:       mode,
					HashPolicy: hashPolicy,
					MIIMon:     ntwrk.Params.BondMiimon,
					UpDelay:    ntwrk.Params.UpDelay,
					DownDelay:  ntwrk.Params.DownDelay,
					LACPRate:   lacpRate,
				},
			}

			if ntwrk.MTU != 0 {
				bondLink.MTU = ntwrk.MTU
			}

			networkadapter.BondMasterSpec(&bondLink.BondMaster).FillDefaults()
			networkConfig.Links = append(networkConfig.Links, bondLink)

			for idx, slave := range ntwrk.BondInterfaces {
				enslavedLinks[slave] = enslavedLink{
					bondName:   name,
					slaveIndex: idx,
				}
			}

			if err = parseSubnets(ntwrk, name); err != nil {
				return false, err
			}
		case "vlan":
			name := ntwrk.Interfaces

			parentName, ok := physicalNameMap[ntwrk.VlanLink]
			if !ok {
				parentName = ntwrk.VlanLink
			}

			linkSpec := network.LinkSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Name:        name,
				Logical:     true,
				Up:          true,
				Kind:        network.LinkKindVLAN,
				Type:        nethelpers.LinkEther,
				ParentName:  parentName,
				VLAN: network.VLANSpec{
					VID:      ntwrk.VlanID,
					Protocol: nethelpers.VLANProtocol8021Q,
				},
			}

			if ntwrk.MTU != 0 {
				linkSpec.MTU = ntwrk.MTU
			}

			networkConfig.Links = append(networkConfig.Links, linkSpec)

			if err = parseSubnets(ntwrk, name); err != nil {
				return false, err
			}
		case "physical":
			name := ntwrk.Interfaces

			if ntwrk.Mac != "" {
				macAddressMatched := false

				for hostInterface := range hostInterfaces.All() {
					if !hostInterface.TypedSpec().Physical() {
						continue
					}

					macAddress := hostInterface.TypedSpec().PermanentAddr.String()
					if macAddress == "" {
						macAddress = hostInterface.TypedSpec().HardwareAddr.String()
					}

					if strings.EqualFold(macAddress, ntwrk.Mac) {
						name = hostInterface.Metadata().ID()
						macAddressMatched = true

						break
					}
				}

				if !macAddressMatched {
					log.Printf("nocloud: no link with matching MAC address %q, defaulted to use name %s instead", ntwrk.Mac, name)

					needsReconcile = true
				}
			}

			physicalNameMap[ntwrk.Interfaces] = name

			linkSpec := network.LinkSpecSpec{
				Name:        name,
				Up:          true,
				ConfigLayer: network.ConfigPlatform,
			}

			if ntwrk.MTU != 0 {
				linkSpec.MTU = ntwrk.MTU
			}

			networkConfig.Links = append(networkConfig.Links, linkSpec)

			if err = parseSubnets(ntwrk, name); err != nil {
				return false, err
			}
		}
	}

	for slaveName, enslavedLink := range enslavedLinks {
		physicalName, ok := physicalNameMap[slaveName]
		if !ok {
			physicalName = slaveName
		}

		for idx := range networkConfig.Links {
			if networkConfig.Links[idx].Name != physicalName {
				continue
			}

			networkConfig.Links[idx].BondSlave = network.BondSlave{
				MasterName: enslavedLink.bondName,
				SlaveIndex: enslavedLink.slaveIndex,
			}
		}
	}

	return needsReconcile, nil
}

//nolint:gocyclo
func applyNetworkConfigV2Ethernet(name string, eth Ethernet, networkConfig *runtime.PlatformNetworkConfig, dnsIPs *[]netip.Addr) error {
	if eth.DHCPv4 {
		networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
			Operator:  network.OperatorDHCP4,
			LinkName:  name,
			RequireUp: true,
			DHCP4: network.DHCP4OperatorSpec{
				RouteMetric: network.DefaultRouteMetric,
			},
			ConfigLayer: network.ConfigPlatform,
		})
	}

	if eth.DHCPv6 {
		networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
			Operator:  network.OperatorDHCP6,
			LinkName:  name,
			RequireUp: true,
			DHCP6: network.DHCP6OperatorSpec{
				RouteMetric: network.DefaultRouteMetric,
			},
			ConfigLayer: network.ConfigPlatform,
		})
	}

	for _, addr := range eth.Address {
		ipPrefix, err := netip.ParsePrefix(addr)
		if err != nil {
			return err
		}

		family := nethelpers.FamilyInet4

		if ipPrefix.Addr().Is6() {
			family = nethelpers.FamilyInet6
		}

		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    name,
				Address:     ipPrefix,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      family,
			},
		)
	}

	if eth.Gateway4 != "" {
		gw, err := netip.ParseAddr(eth.Gateway4)
		if err != nil {
			return err
		}

		route := network.RouteSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Gateway:     gw,
			OutLinkName: name,
			Table:       nethelpers.TableMain,
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Family:      nethelpers.FamilyInet4,
			Priority:    network.DefaultRouteMetric,
		}

		route.Normalize()

		networkConfig.Routes = append(networkConfig.Routes, route)
	}

	if eth.Gateway6 != "" {
		gw, err := netip.ParseAddr(eth.Gateway6)
		if err != nil {
			return err
		}

		route := network.RouteSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Gateway:     gw,
			OutLinkName: name,
			Table:       nethelpers.TableMain,
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Family:      nethelpers.FamilyInet6,
			Priority:    2 * network.DefaultRouteMetric,
		}

		route.Normalize()

		networkConfig.Routes = append(networkConfig.Routes, route)
	}

	for _, addr := range eth.NameServers.Address {
		if ip, err := netip.ParseAddr(addr); err == nil {
			*dnsIPs = append(*dnsIPs, ip)
		} else {
			return err
		}
	}

	for _, route := range eth.Routes {
		gw, err := netip.ParseAddr(route.Via)
		if err != nil {
			return fmt.Errorf("failed to parse route gateway: %w", err)
		}

		dest, err := netip.ParsePrefix(route.To)
		if err != nil {
			return fmt.Errorf("failed to parse route destination: %w", err)
		}

		routeSpec := network.RouteSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Destination: dest,
			Gateway:     gw,
			OutLinkName: name,
			Table:       withDefault(nethelpers.RoutingTable(route.Table), nethelpers.TableMain),
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Family:      nethelpers.FamilyInet4,
			Priority:    withDefault(route.Metric, network.DefaultRouteMetric),
		}

		if gw.Is6() {
			routeSpec.Family = nethelpers.FamilyInet6

			if routeSpec.Priority == network.DefaultRouteMetric {
				routeSpec.Priority = 2 * network.DefaultRouteMetric
			}
		}

		routeSpec.Normalize()

		networkConfig.Routes = append(networkConfig.Routes, routeSpec)

		if route.OnLink && gw.Is4() {
			// This assumes an interface with multiple routes will never have multiple statically set ips.
			ipPrefix, err := netip.ParsePrefix(eth.Address[0])
			if err != nil {
				return fmt.Errorf("failed to parse route source: %w", err)
			}

			routeSpec := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Destination: netip.PrefixFrom(gw, gw.BitLen()),
				Source:      ipPrefix.Addr(),
				OutLinkName: name,
				Scope:       nethelpers.ScopeLink,
				Table:       withDefault(nethelpers.RoutingTable(route.Table), nethelpers.TableMain),
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      nethelpers.FamilyInet4,
				Priority:    withDefault(route.Metric, network.DefaultRouteMetric),
			}

			networkConfig.Routes = append(networkConfig.Routes, routeSpec)
		}
	}

	return nil
}

//nolint:gocyclo,cyclop
func (n *Nocloud) applyNetworkConfigV2(ctx context.Context, config *NetworkConfig, st state.State, networkConfig *runtime.PlatformNetworkConfig) (bool, error) {
	var (
		dnsIPs         []netip.Addr
		needsReconcile bool
	)

	hostInterfaces, err := safe.StateListAll[*network.LinkStatus](ctx, st)
	if err != nil {
		return false, fmt.Errorf("error listing host interfaces: %w", err)
	}

	ethernetNames := maps.Keys(config.Ethernets)
	slices.Sort(ethernetNames)

	for _, name := range ethernetNames {
		eth := config.Ethernets[name]

		var bondSlave network.BondSlave

		for bondName, bond := range config.Bonds {
			for idx, iface := range bond.Interfaces {
				if iface == name {
					bondSlave.MasterName = bondName
					bondSlave.SlaveIndex = idx
				}
			}
		}

		if eth.Match.HWAddr != "" {
			var availableMACAddresses []string

			macAddressMatched := false

			for hostInterface := range hostInterfaces.All() {
				if !hostInterface.TypedSpec().Physical() {
					continue
				}

				macAddress := hostInterface.TypedSpec().PermanentAddr.String()
				if macAddress == "" {
					macAddress = hostInterface.TypedSpec().HardwareAddr.String()
				}

				if strings.EqualFold(macAddress, eth.Match.HWAddr) {
					name = hostInterface.Metadata().ID()
					macAddressMatched = true

					break
				}

				availableMACAddresses = append(availableMACAddresses, macAddress)
			}

			if !macAddressMatched {
				log.Printf("nocloud: no link with matching MAC address %q (available %v), defaulted to use name %s instead", eth.Match.HWAddr, availableMACAddresses, name)

				needsReconcile = true
			}
		}

		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        name,
			Up:          true,
			MTU:         eth.MTU,
			ConfigLayer: network.ConfigPlatform,
			BondSlave:   bondSlave,
		})

		err := applyNetworkConfigV2Ethernet(name, eth, networkConfig, &dnsIPs)
		if err != nil {
			return false, err
		}
	}

	bondNames := maps.Keys(config.Bonds)
	slices.Sort(bondNames)

	for _, bondName := range bondNames {
		bond := config.Bonds[bondName]

		mode, err := nethelpers.BondModeByName(bond.Params.Mode)
		if err != nil {
			return false, fmt.Errorf("invalid mode: %w", err)
		}

		hashPolicy, err := nethelpers.BondXmitHashPolicyByName(bond.Params.HashPolicy)
		if err != nil {
			return false, fmt.Errorf("invalid transmit-hash-policy: %w", err)
		}

		lacpRate, err := nethelpers.LACPRateByName(bond.Params.LACPRate)
		if err != nil {
			return false, fmt.Errorf("invalid lacp-rate: %w", err)
		}

		bondLink := network.LinkSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Name:        bondName,
			Logical:     true,
			Up:          true,
			MTU:         bond.Ethernet.MTU,
			Kind:        network.LinkKindBond,
			Type:        nethelpers.LinkEther,
			BondMaster: network.BondMasterSpec{
				Mode:       mode,
				HashPolicy: hashPolicy,
				MIIMon:     bond.Params.MIIMon,
				UpDelay:    bond.Params.UpDelay,
				DownDelay:  bond.Params.DownDelay,
				LACPRate:   lacpRate,
			},
		}

		networkadapter.BondMasterSpec(&bondLink.BondMaster).FillDefaults()
		networkConfig.Links = append(networkConfig.Links, bondLink)

		err = applyNetworkConfigV2Ethernet(bondName, bond.Ethernet, networkConfig, &dnsIPs)
		if err != nil {
			return false, err
		}
	}

	vlanNames := maps.Keys(config.VLANs)
	slices.Sort(vlanNames)

	for _, vlanName := range vlanNames {
		vlan := config.VLANs[vlanName]

		vlanLink := network.LinkSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Name:        vlanName,
			Logical:     true,
			Up:          true,
			MTU:         vlan.Ethernet.MTU,
			Kind:        network.LinkKindVLAN,
			Type:        nethelpers.LinkEther,
			ParentName:  vlan.Link,
			VLAN: network.VLANSpec{
				VID:      vlan.ID,
				Protocol: nethelpers.VLANProtocol8021Q,
			},
		}

		networkConfig.Links = append(networkConfig.Links, vlanLink)

		err = applyNetworkConfigV2Ethernet(vlanName, vlan.Ethernet, networkConfig, &dnsIPs)
		if err != nil {
			return false, err
		}
	}

	if len(dnsIPs) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	return needsReconcile, nil
}

func withDefault[T comparable](v T, defaultValue T) T {
	var zeroT T

	if v == zeroT {
		return defaultValue
	}

	return v
}

// FetchInclude fetches nocloud #include configuration from the URL specified in the body.
func (n *Nocloud) FetchInclude(ctx context.Context, body []byte, st state.State) ([]byte, error) {
	u, err := ExtractIncludeURL(body)
	if err != nil {
		return nil, err
	}

	log.Printf("fetching the nocloud #include configuration from: %q", u.String())

	if err = netutils.Wait(ctx, st); err != nil {
		return nil, err
	}

	return download.Download(ctx, u.String(), download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// ExtractIncludeURL extracts the URL from the body of a nocloud #include configuration.
//
// Note: only a single URL is expected in the body.
func ExtractIncludeURL(body []byte) (*url.URL, error) {
	var urlLine string

	scanner := bufio.NewScanner(bytes.NewReader(body))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if urlLine != "" {
			return nil, fmt.Errorf("multiple #include URLs found in nocloud configuration: %q and %q", urlLine, line)
		}

		urlLine = line
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if urlLine == "" {
		return nil, stderrors.New("no #include URL found in nocloud configuration")
	}

	return url.Parse(urlLine)
}
