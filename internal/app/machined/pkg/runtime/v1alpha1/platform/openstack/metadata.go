// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack

import (
	"context"
	stderrors "errors"
	"io/fs"
	"log"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/blockutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/xfs"
)

const (
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
	err = blockutils.ReadFromVolume(ctx, r, []string{configISOLabel}, func(root xfs.Root, volumeStatus *block.VolumeStatus) error {
		log.Printf("found config disk (config-drive) at %s", volumeStatus.TypedSpec().Location)

		log.Printf("fetching meta config from: config-drive/%s", configMetadataPath)

		metaConfig, err = xfs.ReadFile(root, configMetadataPath)
		if err != nil {
			log.Printf("failed to read %s", configMetadataPath)

			metaConfig = nil
		}

		log.Printf("fetching network config from: config-drive/%s", configNetworkDataPath)

		networkConfig, err = xfs.ReadFile(root, configNetworkDataPath)
		if err != nil {
			log.Printf("failed to read %s", configNetworkDataPath)

			networkConfig = nil
		}

		log.Printf("fetching machine config from: config-drive/%s", configUserDataPath)

		machineConfig, err = xfs.ReadFile(root, configUserDataPath)
		if err != nil {
			log.Printf("failed to read %s", configUserDataPath)

			machineConfig = nil
		}

		return nil
	})
	if err != nil {
		if stderrors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil, errors.ErrNoConfigSource
		}

		return nil, nil, nil, err
	}

	if machineConfig == nil {
		err = errors.ErrNoConfigSource
	}

	return metaConfig, networkConfig, machineConfig, err
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
