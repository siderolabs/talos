// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud

import (
	"context"
	stderrors "errors"
	"log"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
)

const (
	// HCloudExternalIPEndpoint is the local hcloud endpoint for the external IP.
	HCloudExternalIPEndpoint = "http://169.254.169.254/hetzner/v1/metadata/public-ipv4"

	// HCloudNetworkEndpoint is the local hcloud endpoint for the network-config.
	HCloudNetworkEndpoint = "http://169.254.169.254/hetzner/v1/metadata/network-config"

	// HCloudHostnameEndpoint is the local hcloud endpoint for the hostname.
	HCloudHostnameEndpoint = "http://169.254.169.254/hetzner/v1/metadata/hostname"

	// HCloudInstanceIDEndpoint is the local hcloud endpoint for the instance-id.
	HCloudInstanceIDEndpoint = "http://169.254.169.254/hetzner/v1/metadata/instance-id"

	// HCloudRegionEndpoint is the local hcloud endpoint for the region.
	HCloudRegionEndpoint = "http://169.254.169.254/hetzner/v1/metadata/region"

	// HCloudZoneEndpoint is the local hcloud endpoint for the zone.
	HCloudZoneEndpoint = "http://169.254.169.254/hetzner/v1/metadata/availability-zone"

	// HCloudUserDataEndpoint is the local hcloud endpoint for the config.
	HCloudUserDataEndpoint = "http://169.254.169.254/hetzner/v1/userdata"
)

// MetadataConfig holds meta info.
type MetadataConfig struct {
	Hostname         string `yaml:"hostname,omitempty"`
	Region           string `yaml:"region,omitempty"`
	AvailabilityZone string `json:"availability-zone,omitempty"`
	InstanceID       string `yaml:"instance-id,omitempty"`
	PublicIPv4       string `yaml:"public-ipv4,omitempty"`
}

// NetworkConfig holds hcloud network-config info.
type NetworkConfig struct {
	Version int `yaml:"version"`
	Config  []struct {
		Mac        string `yaml:"mac_address"`
		Interfaces string `yaml:"name"`
		Subnets    []struct {
			NameServers []string `yaml:"dns_nameservers,omitempty"`
			Address     string   `yaml:"address,omitempty"`
			Gateway     string   `yaml:"gateway,omitempty"`
			Ipv4        bool     `yaml:"ipv4,omitempty"`
			Ipv6        bool     `yaml:"ipv6,omitempty"`
			Type        string   `yaml:"type"`
		} `yaml:"subnets"`
		Type string `yaml:"type"`
	} `yaml:"config"`
}

func (h *Hcloud) getMetadata(ctx context.Context) (*MetadataConfig, error) {
	log.Printf("fetching hostname from: %q", HCloudHostnameEndpoint)

	host, err := download.Download(ctx, HCloudHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil && !stderrors.Is(err, errors.ErrNoHostname) {
		return nil, err
	}

	log.Printf("fetching instance-id from: %q", HCloudInstanceIDEndpoint)

	instanceID, err := download.Download(ctx, HCloudInstanceIDEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil && !stderrors.Is(err, errors.ErrNoHostname) {
		return nil, err
	}

	log.Printf("fetching externalIP from: %q", HCloudExternalIPEndpoint)

	extIP, err := download.Download(ctx, HCloudExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil && !stderrors.Is(err, errors.ErrNoExternalIPs) {
		return nil, err
	}

	return &MetadataConfig{
		Hostname:   string(host),
		InstanceID: string(instanceID),
		PublicIPv4: string(extIP),
	}, nil
}
