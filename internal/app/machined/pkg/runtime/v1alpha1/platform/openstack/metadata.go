// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-blockdevice/blockdevice/filesystem"
	"github.com/siderolabs/go-blockdevice/blockdevice/probe"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
)

const (
	// mnt is folder to mount config drive.
	mnt = "/mnt"

	// config-drive configs path.
	configISOLabel        = "config-2"
	configMetadataPath    = "openstack/latest/meta_data.json"
	configNetworkDataPath = "openstack/latest/network_data.json"
	configUserDataPath    = "openstack/latest/user_data"

	endpoint = "http://169.254.169.254/"

	// OpenStackExternalIPEndpoint is the local OpenStack endpoint for the external IP.
	OpenStackExternalIPEndpoint = endpoint + "latest/meta-data/public-ipv4"
	// OpenStackInstanceTypeEndpoint is the local OpenStack endpoint for the instance-type.
	OpenStackInstanceTypeEndpoint = endpoint + "latest/meta-data/instance-type"
	// OpenStackMetaDataEndpoint is the local OpenStack endpoint for the meta config.
	OpenStackMetaDataEndpoint = endpoint + configMetadataPath
	// OpenStackNetworkDataEndpoint is the local OpenStack endpoint for the network config.
	OpenStackNetworkDataEndpoint = endpoint + configNetworkDataPath
	// OpenStackUserDataEndpoint is the local OpenStack endpoint for the config.
	OpenStackUserDataEndpoint = endpoint + configUserDataPath
)

// NetworkConfig holds NetworkData config.
type NetworkConfig struct {
	Links []struct {
		ID             string   `json:"id,omitempty"`
		Type           string   `json:"type"`
		Mac            string   `json:"ethernet_mac_address,omitempty"`
		MTU            int      `json:"mtu,omitempty"`
		BondMode       string   `json:"bond_mode,omitempty"`
		BondLinks      []string `json:"bond_links,omitempty"`
		BondMIIMon     uint32   `json:"bond_miimon,string,omitempty"`
		BondHashPolicy string   `json:"bond_xmit_hash_policy,omitempty"`
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
	UUID             string `json:"uuid,omitempty"`
	Hostname         string `json:"hostname,omitempty"`
	AvailabilityZone string `json:"availability_zone,omitempty"`
	ProjectID        string `json:"project_id"`
	InstanceType     string `json:"instance_type"`
}

func (o *OpenStack) configFromNetwork(ctx context.Context) (metaConfig []byte, networkConfig []byte, machineConfig []byte, err error) {
	log.Printf("fetching meta config from: %q", OpenStackMetaDataEndpoint)

	metaConfig, err = download.Download(ctx, OpenStackMetaDataEndpoint)
	if err != nil {
		metaConfig = nil
	}

	log.Printf("fetching network config from: %q", OpenStackNetworkDataEndpoint)

	networkConfig, err = download.Download(ctx, OpenStackNetworkDataEndpoint)
	if err != nil {
		networkConfig = nil
	}

	log.Printf("fetching machine config from: %q", OpenStackUserDataEndpoint)

	machineConfig, err = download.Download(ctx, OpenStackUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))

	return metaConfig, networkConfig, machineConfig, err
}

//nolint:gocyclo
func (o *OpenStack) configFromCD(ctx context.Context, r state.State) (metaConfig []byte, networkConfig []byte, machineConfig []byte, err error) {
	if err := netutils.WaitForDevicesReady(ctx, r); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to wait for devices: %w", err)
	}

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

	metaConfig, err = os.ReadFile(filepath.Join(mnt, configMetadataPath))
	if err != nil {
		log.Printf("failed to read %s", configMetadataPath)

		metaConfig = nil
	}

	log.Printf("fetching network config from: config-drive/%s", configNetworkDataPath)

	networkConfig, err = os.ReadFile(filepath.Join(mnt, configNetworkDataPath))
	if err != nil {
		log.Printf("failed to read %s", configNetworkDataPath)

		networkConfig = nil
	}

	log.Printf("fetching machine config from: config-drive/%s", configUserDataPath)

	machineConfig, err = os.ReadFile(filepath.Join(mnt, configUserDataPath))
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

func (o *OpenStack) instanceType(ctx context.Context) string {
	log.Printf("fetching instance-type from: %q", OpenStackInstanceTypeEndpoint)

	sku, err := download.Download(ctx, OpenStackInstanceTypeEndpoint)
	if err != nil {
		return ""
	}

	return string(sku)
}

func (o *OpenStack) externalIPs(ctx context.Context) (addrs []netip.Addr) {
	log.Printf("fetching externalIP from: %q", OpenStackExternalIPEndpoint)

	exIP, err := download.Download(ctx, OpenStackExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		log.Printf("failed to fetch external IPs, ignored: %s", err)

		return nil
	}

	if addr, err := netip.ParseAddr(string(exIP)); err == nil {
		addrs = append(addrs, addr)
	}

	return addrs
}
