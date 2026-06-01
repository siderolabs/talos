// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/download"
)

const (
	// HCloudMetadataEndpoint is the local HCloud metadata endpoint.
	HCloudMetadataEndpoint = "http://169.254.169.254/hetzner/v1/metadata"

	// HCloudNetworkEndpoint is the local HCloud metadata endpoint for the network-config.
	HCloudNetworkEndpoint = "http://169.254.169.254/hetzner/v1/metadata/network-config"

	// HCloudUserDataEndpoint is the local HCloud metadata endpoint for the config.
	HCloudUserDataEndpoint = "http://169.254.169.254/hetzner/v1/userdata"
)

// MetadataConfig holds meta info.
type MetadataConfig struct {
	Hostname         string `yaml:"hostname,omitempty"`
	Region           string `yaml:"region,omitempty"`
	AvailabilityZone string `yaml:"availability-zone,omitempty"`
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

func (h *Hcloud) getMetadata(ctx context.Context) (metadata *MetadataConfig, err error) {
	getMetadataKey := func(key string) (string, error) {
		res, metaerr := download.Download(ctx, fmt.Sprintf("%s/%s", HCloudMetadataEndpoint, key),
			download.WithErrorOnNotFound(errors.ErrNoConfigSource),
			download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
		if metaerr != nil && !stderrors.Is(metaerr, errors.ErrNoConfigSource) {
			return "", fmt.Errorf("failed to fetch %q from IMDS: %w", key, metaerr)
		}

		return string(res), nil
	}

	metadata = &MetadataConfig{}

	if metadata.Hostname, err = getMetadataKey("hostname"); err != nil {
		return nil, err
	}

	if metadata.InstanceID, err = getMetadataKey("instance-id"); err != nil {
		return nil, err
	}

	if metadata.AvailabilityZone, err = getMetadataKey("availability-zone"); err != nil {
		return nil, err
	}

	// Original CCM/CSI uses first part of availability-zone to define region name.
	// But metadata has different value.
	// We will follow official behavior.
	metadata.Region = strings.SplitN(metadata.AvailabilityZone, "-", 2)[0]

	if metadata.PublicIPv4, err = getMetadataKey("public-ipv4"); err != nil {
		return nil, err
	}

	return metadata, nil
}
