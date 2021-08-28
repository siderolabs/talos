// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nocloud

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"github.com/talos-systems/go-blockdevice/blockdevice/probe"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-smbios/smbios"
	"golang.org/x/sys/unix"
	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
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

// Nocloud is the concrete type that implements the runtime.Platform interface.
type Nocloud struct{}

// Name implements the runtime.Platform interface.
func (n *Nocloud) Name() string {
	return "nocloud"
}

// ConfigurationNetwork implements the network configuration interface.
//nolint:gocyclo
func (n *Nocloud) ConfigurationNetwork(metadataNetworkConfig []byte, metadataConfig []byte, confProvider config.Provider) (config.Provider, error) {
	var (
		unmarshalledMetadataConfig MetadataConfig
		machineConfig              *v1alpha1.Config
	)

	if err := yaml.Unmarshal(metadataConfig, &unmarshalledMetadataConfig); err != nil {
		unmarshalledMetadataConfig = MetadataConfig{}
	}

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

	if machineConfig.MachineConfig.MachineNetwork.NetworkHostname == "" && unmarshalledMetadataConfig.Hostname != "" {
		machineConfig.MachineConfig.MachineNetwork.NetworkHostname = unmarshalledMetadataConfig.Hostname
	}

	if machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces == nil {
		var unmarshalledNetworkConfig NetworkConfig

		if err := yaml.Unmarshal(metadataNetworkConfig, &unmarshalledNetworkConfig); err != nil {
			return nil, err
		}

		switch unmarshalledNetworkConfig.Version {
		case 1:
			n.applyNetworkConfigV1(unmarshalledNetworkConfig, machineConfig)
		case 2:
			n.applyNetworkConfigV2(unmarshalledNetworkConfig, machineConfig)
		default:
			return nil, fmt.Errorf("network-config metadata version=%d is not supported", unmarshalledNetworkConfig.Version)
		}
	}

	return confProvider, nil
}

// Configuration implements the runtime.Platform interface.
//nolint:gocyclo
func (n *Nocloud) Configuration(ctx context.Context) ([]byte, error) {
	s, err := smbios.New()
	if err != nil {
		return nil, err
	}

	hostname := ""
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

	var (
		metadataConfigDl        []byte
		metadataNetworkConfigDl []byte
		machineConfigDl         []byte
	)

	if networkSource && metaBaseURL != "" {
		metadataConfigDl, metadataNetworkConfigDl, machineConfigDl, err = n.configFromNetwork(ctx, metaBaseURL)
		if err != nil {
			return nil, err
		}
	} else {
		metadataConfigDl, metadataNetworkConfigDl, machineConfigDl, err = n.configFromCD()
		if err != nil {
			return nil, err
		}
	}

	if hostname != "" && metadataConfigDl == nil {
		meta := &MetadataConfig{
			Hostname: hostname,
		}

		//nolint:errcheck
		metadataConfigDl, _ = yaml.Marshal(meta)
	}

	if bytes.HasPrefix(machineConfigDl, []byte("#cloud-config")) {
		return nil, errors.ErrNoConfigSource
	}

	confProvider, err := configloader.NewFromBytes(machineConfigDl)
	if err != nil {
		return nil, err
	}

	confProvider, err = n.ConfigurationNetwork(metadataNetworkConfigDl, metadataConfigDl, confProvider)
	if err != nil {
		return nil, err
	}

	return confProvider.Bytes()
}

// Mode implements the runtime.Platform interface.
func (n *Nocloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (n *Nocloud) Hostname(ctx context.Context) (hostname []byte, err error) {
	return nil, nil
}

// ExternalIPs implements the runtime.Platform interface.
func (n *Nocloud) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	return addrs, nil
}

// KernelArgs implements the runtime.Platform interface.
func (n *Nocloud) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}

//nolint:gocyclo
func (n *Nocloud) applyNetworkConfigV1(networkConfig NetworkConfig, machineConfig *v1alpha1.Config) {
	for _, network := range networkConfig.Config {
		switch network.Type {
		case "nameserver":
			if machineConfig.MachineConfig.MachineNetwork.NameServers == nil {
				machineConfig.MachineConfig.MachineNetwork.NameServers = network.Address
			}
		case "physical":
			iface := v1alpha1.Device{
				DeviceInterface: network.Interfaces,
				DeviceDHCP:      false,
			}

			for _, subnet := range network.Subnets {
				switch subnet.Type {
				case "dhcp", "dhcp4":
					iface.DeviceDHCP = true
				case "static":
					cidr := strings.SplitN(subnet.Address, "/", 2)
					if len(cidr) == 2 {
						iface.DeviceAddresses = append(iface.DeviceAddresses,
							subnet.Address,
						)
					} else {
						mask, _ := net.IPMask(net.ParseIP(subnet.Netmask).To4()).Size()

						iface.DeviceAddresses = append(iface.DeviceAddresses,
							fmt.Sprintf("%s/%d", subnet.Address, mask),
						)
					}

					if subnet.Gateway != "" {
						iface.DeviceRoutes = append(iface.DeviceRoutes, &v1alpha1.Route{
							RouteNetwork: "0.0.0.0/0",
							RouteGateway: subnet.Gateway,
							RouteMetric:  1024,
						})
					}
				case "static6":
					iface.DeviceAddresses = append(iface.DeviceAddresses,
						subnet.Address,
					)
					if subnet.Gateway != "" {
						iface.DeviceRoutes = append(iface.DeviceRoutes, &v1alpha1.Route{
							RouteNetwork: "::/0",
							RouteGateway: subnet.Gateway,
							RouteMetric:  1024,
						})
					}
				case "ipv6_dhcpv6-stateful":
					iface.DeviceDHCPOptions = &v1alpha1.DHCPOptions{
						DHCPIPv6:        pointer.ToBool(true),
						DHCPRouteMetric: 1024,
					}
				}
			}

			machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(
				machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces,
				&iface,
			)
		}
	}
}

//nolint:gocyclo
func (n *Nocloud) applyNetworkConfigV2(networkConfig NetworkConfig, machineConfig *v1alpha1.Config) {
	var ns []string

	for name, eth := range networkConfig.Ethernets {
		if !strings.HasPrefix(name, "eth") {
			continue
		}

		iface := v1alpha1.Device{
			DeviceInterface: name,
			DeviceDHCP:      eth.DHCPv4,
		}

		if eth.DHCPv6 {
			iface.DeviceDHCPOptions = &v1alpha1.DHCPOptions{
				DHCPIPv6:        pointer.ToBool(true),
				DHCPRouteMetric: 1024,
			}
		}

		if eth.Address != nil {
			iface.DeviceAddresses = append(iface.DeviceAddresses, eth.Address...)
		}

		if eth.Gateway4 != "" {
			iface.DeviceRoutes = append(iface.DeviceRoutes, &v1alpha1.Route{
				RouteNetwork: "0.0.0.0/0",
				RouteGateway: eth.Gateway4,
				RouteMetric:  1024,
			})
		}

		if eth.Gateway6 != "" {
			iface.DeviceRoutes = append(iface.DeviceRoutes, &v1alpha1.Route{
				RouteNetwork: "::/0",
				RouteGateway: eth.Gateway6,
				RouteMetric:  1024,
			})
		}

		if eth.MTU != 0 {
			iface.DeviceMTU = eth.MTU
		}

		if eth.NameServers.Address != nil {
			ns = append(ns, eth.NameServers.Address...)
		}

		machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(
			machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces,
			&iface,
		)
	}

	if machineConfig.MachineConfig.MachineNetwork.NameServers == nil && ns != nil {
		machineConfig.MachineConfig.MachineNetwork.NameServers = ns
	}
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
	if err != nil {
		return nil, nil, nil, err
	}

	return metaConfig, networkConfig, machineConfig, nil
}

//nolint:gocyclo
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

	if machineConfig == nil {
		return nil, nil, nil, errors.ErrNoConfigSource
	}

	return metaConfig, networkConfig, machineConfig, nil
}
