// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"

	"github.com/siderolabs/talos/pkg/download"
)

// Metadata extends the SDK's instance.Metadata with fields not yet present in the SDK.
type Metadata struct {
	instance.Metadata

	// RoutedIPEnabled is true when the instance is in routed IP mode. In this mode
	// IPs from PublicIpsV4/V6 are configured statically rather than via DHCP.
	RoutedIPEnabled bool `json:"routed_ip_enabled"`
}

const (
	// ScalewayMetadataEndpoint is the local Scaleway IPv4 metadata endpoint.
	ScalewayMetadataEndpoint = "http://169.254.42.42/conf?format=json"
	// ScalewayMetadataEndpointIPv6 is the local Scaleway IPv6 metadata endpoint.
	ScalewayMetadataEndpointIPv6 = "http://[fd00:42::42]/conf?format=json"
	// ScalewayUserDataEndpoint is the local Scaleway IPv4 endpoint for the config.
	ScalewayUserDataEndpoint = "http://169.254.42.42/user_data/cloud-init"
	// ScalewayUserDataEndpointIPv6 is the local Scaleway IPv6 endpoint for the config.
	ScalewayUserDataEndpointIPv6 = "http://[fd00:42::42]/user_data/cloud-init"

	// metadataIPv4Timeout is how long to wait for the IPv4 metadata endpoint before
	// falling back to IPv6. Long enough for dual-stack instances where IPv4 needs a
	// few seconds to come up, short enough not to delay IPv6-only instances too much.
	metadataIPv4Timeout = 15 * time.Second
)

func (u *Scaleway) getMetadata(ctx context.Context) (*Metadata, error) {
	probeCtx, cancel := context.WithTimeout(ctx, metadataIPv4Timeout)
	metaConfigDl, err := download.Download(probeCtx, ScalewayMetadataEndpoint,
		download.WithTimeout(metadataIPv4Timeout))

	cancel()

	if err != nil {
		log.Printf("IPv4 metadata unreachable (%s), falling back to IPv6", err)

		metaConfigDl, err = download.Download(ctx, ScalewayMetadataEndpointIPv6)
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching metadata: %w", err)
	}

	var meta Metadata
	if err = json.Unmarshal(metaConfigDl, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
