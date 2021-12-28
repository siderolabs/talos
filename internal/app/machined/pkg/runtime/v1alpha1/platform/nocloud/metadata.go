// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nocloud

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"github.com/talos-systems/go-blockdevice/blockdevice/probe"
	"github.com/talos-systems/go-smbios/smbios"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

const (
	configISOLabel          = "cidata"
	configNetworkConfigPath = "network-config"
	configMetaDataPath      = "meta-data"
	configUserDataPath      = "user-data"
	mnt                     = "/mnt"
)

// NetworkConfig holds network-config info.
type NetworkConfig struct {
	Version int `yaml:"version"`
	Config  []struct {
		Mac        string `yaml:"mac_address,omitempty"`
		Interfaces string `yaml:"name,omitempty"`
		MTU        string `yaml:"mtu,omitempty"`
		Subnets    []struct {
			Address string `yaml:"address,omitempty"`
			Netmask string `yaml:"netmask,omitempty"`
			Gateway string `yaml:"gateway,omitempty"`
			Type    string `yaml:"type"`
		} `yaml:"subnets,omitempty"`
		Address []string `yaml:"address,omitempty"`
		Type    string   `yaml:"type"`
	} `yaml:"config,omitempty"`
	Ethernets map[string]Ethernet `yaml:"ethernets,omitempty"`
	Bonds     map[string]Bonds    `yaml:"bonds,omitempty"`
}

// Ethernet holds network interface info.
type Ethernet struct {
	Match struct {
		Name   []string `yaml:"name,omitempty"`
		HWAddr []string `yaml:"macaddress,omitempty"`
	} `yaml:"match,omitempty"`
	DHCPv4      bool     `yaml:"dhcp4,omitempty"`
	DHCPv6      bool     `yaml:"dhcp6,omitempty"`
	Address     []string `yaml:"addresses,omitempty"`
	Gateway4    string   `yaml:"gateway4,omitempty"`
	Gateway6    string   `yaml:"gateway6,omitempty"`
	MTU         int      `yaml:"mtu,omitempty"`
	NameServers struct {
		Search  []string `yaml:"search,omitempty"`
		Address []string `yaml:"addresses,omitempty"`
	} `yaml:"nameservers,omitempty"`
}

// Bonds holds bonding interface info.
type Bonds struct {
	Interfaces  []string `yaml:"interfaces,omitempty"`
	Address     []string `yaml:"addresses,omitempty"`
	NameServers struct {
		Search  []string `yaml:"search,omitempty"`
		Address []string `yaml:"addresses,omitempty"`
	} `yaml:"nameservers,omitempty"`
	Params []struct {
		Mode       string `yaml:"mode,omitempty"`
		LACPRate   string `yaml:"lacp-rate,omitempty"`
		HashPolicy string `yaml:"transmit-hash-policy,omitempty"`
	} `yaml:"parameters,omitempty"`
}

// MetadataConfig holds meta info.
type MetadataConfig struct {
	Hostname   string `yaml:"hostname,omitempty"`
	InstanceID string `yaml:"instance-id,omitempty"`
}

func (n *Nocloud) configFromNetwork(ctx context.Context, metaBaseURL string) (metaConfig []byte, networkConfig []byte, machineConfig []byte, err error) {
	log.Printf("fetching meta config from: %q", metaBaseURL+configMetaDataPath)

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

func (n *Nocloud) configFromCD() (metaConfig []byte, networkConfig []byte, machineConfig []byte, err error) {
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

	metaConfig, err = ioutil.ReadFile(filepath.Join(mnt, configMetaDataPath))
	if err != nil {
		log.Printf("failed to read %s", configMetaDataPath)

		metaConfig = nil
	}

	log.Printf("fetching network config from: cidata/%s", configNetworkConfigPath)

	networkConfig, err = ioutil.ReadFile(filepath.Join(mnt, configNetworkConfigPath))
	if err != nil {
		log.Printf("failed to read %s", configNetworkConfigPath)

		networkConfig = nil
	}

	log.Printf("fetching machine config from: cidata/%s", configUserDataPath)

	machineConfig, err = ioutil.ReadFile(filepath.Join(mnt, configUserDataPath))
	if err != nil {
		log.Printf("failed to read %s", configUserDataPath)

		machineConfig = nil
	}

	if err = unix.Unmount(mnt, 0); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to unmount: %w", err)
	}

	return metaConfig, networkConfig, machineConfig, nil
}

//nolint:gocyclo
func (n *Nocloud) acquireConfig(ctx context.Context) (metadataConfigDl, metadataNetworkConfigDl, machineConfigDl []byte, hostname string, err error) {
	s, err := smbios.New()
	if err != nil {
		return nil, nil, nil, "", err
	}

	metaBaseURL := ""
	networkSource := false

	options := strings.Split(s.SystemInformation().SerialNumber(), ";")
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
			}
		}
	}

	if networkSource && metaBaseURL != "" {
		metadataConfigDl, metadataNetworkConfigDl, machineConfigDl, err = n.configFromNetwork(ctx, metaBaseURL)
	} else {
		metadataConfigDl, metadataNetworkConfigDl, machineConfigDl, err = n.configFromCD()
	}

	return metadataConfigDl, metadataNetworkConfigDl, machineConfigDl, hostname, err
}

//nolint:gocyclo
func (n *Nocloud) applyNetworkConfigV1(config *NetworkConfig, networkConfig *runtime.PlatformNetworkConfig) error {
	for _, ntwrk := range config.Config {
		switch ntwrk.Type {
		case "nameserver":
			dnsIPs := make([]netaddr.IP, 0, len(ntwrk.Address))

			for i := range ntwrk.Address {
				if ip, err := netaddr.ParseIP(ntwrk.Address[i]); err == nil {
					dnsIPs = append(dnsIPs, ip)
				} else {
					return err
				}
			}

			networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
				DNSServers:  dnsIPs,
				ConfigLayer: network.ConfigPlatform,
			})
		case "physical":
			networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
				Name:        ntwrk.Interfaces,
				Up:          true,
				ConfigLayer: network.ConfigPlatform,
			})

			for _, subnet := range ntwrk.Subnets {
				switch subnet.Type {
				case "dhcp", "dhcp4":
					networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
						Operator:  network.OperatorDHCP4,
						LinkName:  ntwrk.Interfaces,
						RequireUp: true,
						DHCP4: network.DHCP4OperatorSpec{
							RouteMetric: 1024,
						},
						ConfigLayer: network.ConfigPlatform,
					})
				case "static", "static6":
					family := nethelpers.FamilyInet4

					if subnet.Type == "static6" {
						family = nethelpers.FamilyInet6
					}

					ipPrefix, err := netaddr.ParseIPPrefix(subnet.Address)
					if err != nil {
						ip, err := netaddr.ParseIP(subnet.Address)
						if err != nil {
							return err
						}

						netmask, err := netaddr.ParseIP(subnet.Netmask)
						if err != nil {
							return err
						}

						mask := netmask.As4()

						ipPrefix, err = ip.Netmask(mask[:])
						if err != nil {
							return err
						}
					}

					networkConfig.Addresses = append(networkConfig.Addresses,
						network.AddressSpecSpec{
							ConfigLayer: network.ConfigPlatform,
							LinkName:    ntwrk.Interfaces,
							Address:     ipPrefix,
							Scope:       nethelpers.ScopeGlobal,
							Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
							Family:      family,
						},
					)

					if subnet.Gateway != "" {
						gw, err := netaddr.ParseIP(subnet.Gateway)
						if err != nil {
							return err
						}

						route := network.RouteSpecSpec{
							ConfigLayer: network.ConfigPlatform,
							Gateway:     gw,
							OutLinkName: ntwrk.Interfaces,
							Table:       nethelpers.TableMain,
							Protocol:    nethelpers.ProtocolStatic,
							Type:        nethelpers.TypeUnicast,
							Family:      family,
							Priority:    1024,
						}

						if family == nethelpers.FamilyInet6 {
							route.Priority = 2048
						}

						route.Normalize()

						networkConfig.Routes = append(networkConfig.Routes, route)
					}
				case "ipv6_dhcpv6-stateful":
					networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
						Operator:  network.OperatorDHCP6,
						LinkName:  ntwrk.Interfaces,
						RequireUp: true,
						DHCP6: network.DHCP6OperatorSpec{
							RouteMetric: 1024,
						},
						ConfigLayer: network.ConfigPlatform,
					})
				}
			}
		}
	}

	return nil
}

//nolint:gocyclo
func (n *Nocloud) applyNetworkConfigV2(config *NetworkConfig, networkConfig *runtime.PlatformNetworkConfig) error {
	var dnsIPs []netaddr.IP

	for name, eth := range config.Ethernets {
		if !strings.HasPrefix(name, "eth") {
			continue
		}

		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        name,
			Up:          true,
			MTU:         uint32(eth.MTU),
			ConfigLayer: network.ConfigPlatform,
		})

		if eth.DHCPv4 {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  name,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: 1024,
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
					RouteMetric: 1024,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		}

		for _, addr := range eth.Address {
			ipPrefix, err := netaddr.ParseIPPrefix(addr)
			if err != nil {
				return err
			}

			family := nethelpers.FamilyInet4

			if ipPrefix.IP().Is6() {
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
			gw, err := netaddr.ParseIP(eth.Gateway4)
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
				Priority:    1024,
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		}

		if eth.Gateway6 != "" {
			gw, err := netaddr.ParseIP(eth.Gateway6)
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
				Priority:    2048,
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		}

		for _, addr := range eth.NameServers.Address {
			if ip, err := netaddr.ParseIP(addr); err == nil {
				dnsIPs = append(dnsIPs, ip)
			} else {
				return err
			}
		}
	}

	if len(dnsIPs) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	return nil
}
