// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"github.com/talos-systems/go-blockdevice/blockdevice/probe"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
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
		ID   string `json:"id,omitempty"`
		Type string `json:"type"`
		Mac  string `json:"ethernet_mac_address,omitempty"`
		MTU  int    `json:"mtu,omitempty"`
	} `json:"links"`
	Networks []struct {
		ID      string `json:"id,omitempty"`
		Link    string `json:"link"`
		Type    string `json:"type"`
		Address string `json:"ip_address,omitempty"`
		Netmask string `json:"netmask,omitempty"`
		Gateway string `json:"gateway,omitempty"`
		Routes  []struct {
			Network string `json:"network,omitempty"`
			Netmask string `json:"netmask,omitempty"`
			Gateway string `json:"gateway,omitempty"`
		} `json:"routes,omitempty"`
	} `json:"networks"`
	Services []struct {
		Type    string `json:"type"`
		Address string `json:"address"`
	} `json:"services,omitempty"`
}

// MetadataConfig holds meta info.
type MetadataConfig struct {
	Hostname string `json:"hostname,omitempty"`
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

	return metaConfig, networkConfig, machineConfig, err
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
		return metaConfig, networkConfig, machineConfig, errors.ErrNoConfigSource
	}

	return metaConfig, networkConfig, machineConfig, nil
}

func (o *Openstack) hostname(ctx context.Context) []byte {
	log.Printf("fetching hostname from: %q", OpenstackHostnameEndpoint)

	hostname, err := download.Download(ctx, OpenstackHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		// Platform cannot support this endpoint, or returns timeout.
		log.Printf("failed to fetch hostname, ignored: %s", err)

		return nil
	}

	return hostname
}

func (o *Openstack) externalIPs(ctx context.Context) (addrs []netaddr.IP) {
	log.Printf("fetching externalIP from: %q", OpenstackExternalIPEndpoint)

	exIP, err := download.Download(ctx, OpenstackExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		log.Printf("failed to fetch external IPs, ignored: %s", err)

		return nil
	}

	if addr, err := netaddr.ParseIP(string(exIP)); err == nil {
		addrs = append(addrs, addr)
	}

	return addrs
}
