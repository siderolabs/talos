// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package upcloud

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/siderolabs/talos/pkg/download"
)

const (
	// UpCloudMetadataEndpoint is the local UpCloud endpoint.
	UpCloudMetadataEndpoint = "http://169.254.169.254/metadata/v1.json"

	// UpCloudUserDataEndpoint is the local UpCloud endpoint for the config.
	UpCloudUserDataEndpoint = "http://169.254.169.254/metadata/v1/user_data"
)

// MetadataConfig represents a metadata Upcloud instance.
type MetadataConfig struct {
	Hostname   string   `json:"hostname,omitempty"`
	InstanceID string   `json:"instance_id,omitempty"`
	PublicKeys []string `json:"public_keys,omitempty"`
	Zone       string   `json:"region,omitempty"`
	Tags       []string `json:"tags,omitempty"`

	Network struct {
		Interfaces []struct {
			Index       int `json:"index,omitempty"`
			IPAddresses []struct {
				Address  string   `json:"address,omitempty"`
				DHCP     bool     `json:"dhcp,omitempty"`
				DNS      []string `json:"dns,omitempty"`
				Family   string   `json:"family,omitempty"`
				Floating bool     `json:"floating,omitempty"`
				Gateway  string   `json:"gateway,omitempty"`
				Network  string   `json:"network,omitempty"`
			} `json:"ip_addresses,omitempty"`
			MAC         string `json:"mac,omitempty"`
			NetworkType string `json:"type,omitempty"`
			NetworkID   string `json:"network_id,omitempty"`
		} `json:"interfaces,omitempty"`
		DNS []string `json:"dns,omitempty"`
	} `json:"network,omitempty"`
}

func (u *UpCloud) getMetadata(ctx context.Context) (*MetadataConfig, error) {
	metaConfigDl, err := download.Download(ctx, UpCloudMetadataEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error fetching metadata: %w", err)
	}

	var meta MetadataConfig
	if err = json.Unmarshal(metaConfigDl, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
