// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package digitalocean

import (
	"context"
	"encoding/json"
	stderrors "errors"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/download"
)

const (
	// DigitalOceanExternalIPEndpoint displays all external addresses associated with the instance.
	DigitalOceanExternalIPEndpoint = "http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address"
	// DigitalOceanMetadataEndpoint is the local endpoint for the platform metadata.
	DigitalOceanMetadataEndpoint = "http://169.254.169.254/metadata/v1.json"
	// DigitalOceanUserDataEndpoint is the local endpoint for the config.
	DigitalOceanUserDataEndpoint = "http://169.254.169.254/metadata/v1/user-data"
)

// MetadataConfig represents a metadata Digital Ocean instance.
type MetadataConfig struct {
	Hostname   string   `json:"hostname,omitempty"`
	DropletID  int      `json:"droplet_id,omitempty"`
	Region     string   `json:"region,omitempty"`
	PublicIPv4 string   `json:"public-ipv4,omitempty"`
	Tags       []string `json:"tags,omitempty"`

	DNS struct {
		Nameservers []string `json:"nameservers,omitempty"`
	} `json:"dns"`
	Interfaces map[string][]struct {
		MACAddress string `json:"mac,omitempty"`
		Type       string `json:"type,omitempty"`

		IPv4 *struct {
			IPAddress string `json:"ip_address,omitempty"`
			Netmask   string `json:"netmask,omitempty"`
			Gateway   string `json:"gateway,omitempty"`
		} `json:"ipv4,omitempty"`
		IPv6 *struct {
			IPAddress string `json:"ip_address,omitempty"`
			CIDR      int    `json:"cidr,omitempty"`
			Gateway   string `json:"gateway,omitempty"`
		} `json:"ipv6,omitempty"`
		AnchorIPv4 *struct {
			IPAddress string `json:"ip_address,omitempty"`
			Netmask   string `json:"netmask,omitempty"`
			Gateway   string `json:"gateway,omitempty"`
		} `json:"anchor_ipv4,omitempty"`
	} `json:"interfaces,omitempty"`
}

func (d *DigitalOcean) getMetadata(ctx context.Context) (*MetadataConfig, error) {
	metaConfigDl, err := download.Download(ctx, DigitalOceanMetadataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil && !stderrors.Is(err, errors.ErrNoHostname) {
		return nil, err
	}

	var metadata MetadataConfig
	if err = json.Unmarshal(metaConfigDl, &metadata); err != nil {
		return nil, err
	}

	extIP, err := download.Download(ctx, DigitalOceanExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil && !stderrors.Is(err, errors.ErrNoExternalIPs) {
		return nil, err
	}

	metadata.PublicIPv4 = string(extIP)

	return &metadata, nil
}
