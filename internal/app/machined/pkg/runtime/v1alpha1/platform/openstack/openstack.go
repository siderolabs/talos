// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"github.com/talos-systems/go-blockdevice/blockdevice/probe"
	"github.com/talos-systems/go-procfs/procfs"
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
	// mnt is folder to mount config drive.
	mnt = "/mnt"

	// config-drive configs path.
	configISOLabel        = "config-2"
	configMetadataPath    = "openstack/latest/meta_data.json"
	configNetworkDataPath = "openstack/latest/network_data.json"
	configUserDataPath    = "openstack/latest/user_data"

	// OpenstackExternalIPEndpoint is the local Openstack endpoint for the external IP.
	OpenstackExternalIPEndpoint = "http://169.254.169.254/latest/meta-data/public-ipv4"
	// OpenstackHostnameEndpoint is the local Openstack endpoint for the hostname.
	OpenstackHostnameEndpoint = "http://169.254.169.254/latest/meta-data/hostname"
	// OpenstackMetaDataEndpoint is the local Openstack endpoint for the meta config.
	OpenstackMetaDataEndpoint = "http://169.254.169.254/" + configMetadataPath
	// OpenstackNetworkDataEndpoint is the local Openstack endpoint for the network config.
	OpenstackNetworkDataEndpoint = "http://169.254.169.254/" + configNetworkDataPath
	// OpenstackUserDataEndpoint is the local Openstack endpoint for the config.
	OpenstackUserDataEndpoint = "http://169.254.169.254/" + configUserDataPath
)

// NetworkConfig holds NetworkData config.
type NetworkConfig struct {
	Links []struct {
		ID   string `yaml:"id,omitempty"`
		Type string `yaml:"type"`
		Mac  string `yaml:"ethernet_mac_address,omitempty"`
		MTU  int    `yaml:"mtu,omitempty"`
	} `yaml:"links"`
	Networks []struct {
		ID      string `yaml:"id,omitempty"`
		Link    string `yaml:"link"`
		Type    string `yaml:"type"`
		Address string `yaml:"ip_address,omitempty"`
		Netmask string `yaml:"netmask,omitempty"`
		Gateway string `yaml:"gateway,omitempty"`
		Routes  []struct {
			Network string `yaml:"network,omitempty"`
			Netmask string `yaml:"netmask,omitempty"`
			Gateway string `yaml:"gateway,omitempty"`
		} `yaml:"routes,omitempty"`
	} `yaml:"networks"`
	Services []struct {
		Type    string `yaml:"type"`
		Address string `yaml:"address"`
	} `yaml:"services,omitempty"`
}

// MetadataConfig holds meta info.
type MetadataConfig struct {
	Hostname string `yaml:"hostname,omitempty"`
}

// Openstack is the concrete type that implements the runtime.Platform interface.
type Openstack struct{}

// Name implements the runtime.Platform interface.
func (o *Openstack) Name() string {
	return "openstack"
}

// ConfigurationNetwork implements the network configuration interface.
//nolint:gocyclo,cyclop
func (o *Openstack) ConfigurationNetwork(metadataNetworkConfig []byte, metadataConfig []byte, confProvider config.Provider) (config.Provider, error) {
	var unmarshalledMetadataConfig MetadataConfig

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

		nameServers := []string{}

		for _, netsvc := range unmarshalledNetworkConfig.Services {
			if netsvc.Type == "dns" {
				nameServers = append(nameServers, netsvc.Address)
			}
		}

		if machineConfig.MachineConfig.MachineNetwork.NameServers == nil && len(nameServers) > 0 {
			machineConfig.MachineConfig.MachineNetwork.NameServers = nameServers
		}

		ifaces := make(map[string]*v1alpha1.Device)

		for idx, netLinks := range unmarshalledNetworkConfig.Links {
			switch netLinks.Type {
			case "phy", "vif", "ovs":
				iface := &v1alpha1.Device{
					// We need to define name of interface by MAC
					// I hope it will solve after https://github.com/talos-systems/talos/issues/4203, https://github.com/talos-systems/talos/issues/3265
					DeviceInterface: fmt.Sprintf("eth%d", idx),
					DeviceMTU:       netLinks.MTU,
				}
				ifaces[netLinks.ID] = iface
			}
		}

		for _, network := range unmarshalledNetworkConfig.Networks {
			if network.ID == "" || ifaces[network.Link] == nil {
				continue
			}

			iface := ifaces[network.Link]

			switch network.Type {
			case "ipv4_dhcp":
				iface.DeviceDHCP = true
			case "ipv4":
				cidr := strings.SplitN(network.Address, "/", 2)
				if len(cidr) == 1 {
					mask, err := strconv.Atoi(network.Netmask)
					if err != nil {
						mask, _ = net.IPMask(network.Netmask).Size()
					}

					iface.DeviceAddresses = append(iface.DeviceAddresses, fmt.Sprintf("%s/%d", network.Address, mask))
				} else {
					iface.DeviceAddresses = append(iface.DeviceAddresses, network.Address)
				}

				if network.Gateway != "" {
					iface.DeviceRoutes = append(iface.DeviceRoutes, &v1alpha1.Route{
						RouteNetwork: "0.0.0.0/0",
						RouteGateway: network.Gateway,
						RouteMetric:  1024,
					})
				}
			case "ipv6":
				cidr := strings.SplitN(network.Address, "/", 2)
				if len(cidr) == 1 {
					mask, err := strconv.Atoi(network.Netmask)
					if err != nil {
						mask, _ = net.IPMask(net.ParseIP(network.Netmask).To16()).Size()
					}

					iface.DeviceAddresses = append(iface.DeviceAddresses, fmt.Sprintf("%s/%d", network.Address, mask))
				} else {
					iface.DeviceAddresses = append(iface.DeviceAddresses, network.Address)
				}

				if network.Gateway != "" {
					iface.DeviceRoutes = append(iface.DeviceRoutes, &v1alpha1.Route{
						RouteNetwork: "::/0",
						RouteGateway: network.Gateway,
						RouteMetric:  1024,
					})
				}
			}

			for _, route := range network.Routes {
				mask, err := strconv.Atoi(route.Netmask)
				if err != nil {
					gw := net.ParseIP(route.Network)

					if len(gw) == net.IPv4len {
						mask, _ = net.IPMask(net.ParseIP(route.Netmask).To4()).Size()
					} else {
						mask, _ = net.IPMask(net.ParseIP(route.Netmask).To16()).Size()
					}
				}

				iface.DeviceRoutes = append(iface.DeviceRoutes, &v1alpha1.Route{
					RouteNetwork: fmt.Sprintf("%s/%d", route.Network, mask),
					RouteGateway: route.Gateway,
					RouteMetric:  1024,
				})
			}
		}

		ifaceNames := make([]string, 0, len(ifaces))

		for ifaceName := range ifaces {
			ifaceNames = append(ifaceNames, ifaceName)
		}

		sort.Strings(ifaceNames)

		for _, ifaceName := range ifaceNames {
			machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces = append(machineConfig.MachineConfig.MachineNetwork.NetworkInterfaces, ifaces[ifaceName])
		}

		if machineConfig.MachineConfig.MachineNetwork.NameServers == nil && len(nameServers) > 0 {
			machineConfig.MachineConfig.MachineNetwork.NameServers = nameServers
		}
	}

	return confProvider, nil
}

// Configuration implements the runtime.Platform interface.
func (o *Openstack) Configuration(ctx context.Context) (machineConfig []byte, err error) {
	var (
		metadataConfigDl        []byte
		metadataNetworkConfigDl []byte
	)

	metadataConfigDl, metadataNetworkConfigDl, machineConfig, err = o.configFromCD()
	if err != nil {
		metadataConfigDl, metadataNetworkConfigDl, machineConfig, err = o.configFromNetwork(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Some openstack setups does not allow you to change user-data,
	// so skip this case.
	if bytes.HasPrefix(machineConfig, []byte("#cloud-config")) {
		return nil, errors.ErrNoConfigSource
	}

	confProvider, err := configloader.NewFromBytes(machineConfig)
	if err != nil {
		return nil, err
	}

	confProvider, err = o.ConfigurationNetwork(metadataNetworkConfigDl, metadataConfigDl, confProvider)
	if err != nil {
		return nil, err
	}

	return confProvider.Bytes()
}

// Mode implements the runtime.Platform interface.
func (o *Openstack) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (o *Openstack) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching hostname from: %q", OpenstackHostnameEndpoint)

	hostname, err = download.Download(ctx, OpenstackHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		// Platform cannot support this endpoint, or return timeout.
		log.Printf("failed to fetch hostname, ignored: %s", err)

		return nil, nil
	}

	return hostname, nil
}

// ExternalIPs implements the runtime.Platform interface.
func (o *Openstack) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	log.Printf("fetching externalIP from: %q", OpenstackExternalIPEndpoint)

	exIP, err := download.Download(ctx, OpenstackExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		return nil, err
	}

	if addr := net.ParseIP(string(exIP)); addr != nil {
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

// KernelArgs implements the runtime.Platform interface.
func (o *Openstack) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}

func (o *Openstack) configFromNetwork(ctx context.Context) (metaConfig []byte, networkConfig []byte, machineConfig []byte, err error) {
	log.Printf("fetching meta config from: %q", OpenstackMetaDataEndpoint)

	metaConfig, err = download.Download(ctx, OpenstackMetaDataEndpoint)
	if err != nil {
		metaConfig = nil
	}

	log.Printf("fetching network config from: %q", OpenstackNetworkDataEndpoint)

	networkConfig, err = download.Download(ctx, OpenstackNetworkDataEndpoint)
	if err != nil {
		networkConfig = nil
	}

	log.Printf("fetching machine config from: %q", OpenstackUserDataEndpoint)

	machineConfig, err = download.Download(ctx, OpenstackUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, nil, nil, errors.ErrNoConfigSource
	}

	return metaConfig, networkConfig, machineConfig, nil
}

func (o *Openstack) configFromCD() (metaConfig []byte, networkConfig []byte, machineConfig []byte, err error) {
	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(configISOLabel)
	if err != nil {
		return nil, nil, nil, errors.ErrNoConfigSource
	}

	//nolint:errcheck
	defer dev.Close()

	sb, err := filesystem.Probe(dev.Path)
	if err != nil || sb == nil {
		return nil, nil, nil, errors.ErrNoConfigSource
	}

	log.Printf("found config disk (config-drive) at %s", dev.Path)

	if err = unix.Mount(dev.Path, mnt, sb.Type(), unix.MS_RDONLY, ""); err != nil {
		return nil, nil, nil, errors.ErrNoConfigSource
	}

	log.Printf("fetching meta config from: config-drive/%s", configMetadataPath)

	metaConfig, err = ioutil.ReadFile(filepath.Join(mnt, configMetadataPath))
	if err != nil {
		log.Printf("failed to read %s", configMetadataPath)

		metaConfig = nil
	}

	log.Printf("fetching network config from: config-drive/%s", configNetworkDataPath)

	networkConfig, err = ioutil.ReadFile(filepath.Join(mnt, configNetworkDataPath))
	if err != nil {
		log.Printf("failed to read %s", configNetworkDataPath)

		networkConfig = nil
	}

	log.Printf("fetching machine config from: config-drive/%s", configUserDataPath)

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
