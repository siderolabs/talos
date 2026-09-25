// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/pkg/download"
)

const (
	// ScalewayMetadataEndpoint is the local Scaleway IPv4 metadata endpoint.
	ScalewayMetadataEndpoint = "http://169.254.42.42/conf?format=json"
	// ScalewayMetadataEndpointIPv6 is the local Scaleway IPv6 metadata endpoint.
	ScalewayMetadataEndpointIPv6 = "http://[fd00:42::42]/conf?format=json"
	// ScalewayUserDataEndpoint is the local Scaleway IPv4 endpoint for the config.
	ScalewayUserDataEndpoint = "http://169.254.42.42/user_data/cloud-init"
	// ScalewayUserDataEndpointIPv6 is the local Scaleway IPv6 endpoint for the config.
	ScalewayUserDataEndpointIPv6 = "http://[fd00:42::42]/user_data/cloud-init"

	// endpointAttemptTimeout bounds a single attempt against one address family. An instance
	// that only has one of them fails over to the other after this long, rather than spending
	// the whole retry budget on an endpoint it can never reach.
	endpointAttemptTimeout = 5 * time.Second
)

// metadataRoute is the host route to the metadata service, needed to reach it before any
// address is configured on the link.
var metadataRoute = netip.MustParsePrefix("169.254.42.42/32")

// downloadAlternating retries against the IPv4 and IPv6 metadata endpoints in turn.
func downloadAlternating(ctx context.Context, ipv4Endpoint, ipv6Endpoint string, options ...download.Option) ([]byte, error) {
	endpoints := [...]string{ipv4Endpoint, ipv6Endpoint}
	attempt := 0

	options = append(options,
		download.WithEndpointFunc(func(context.Context) (string, error) {
			endpoint := endpoints[attempt%len(endpoints)]
			attempt++

			return endpoint, nil
		}),
		download.WithRetryOptions(retry.WithAttemptTimeout(endpointAttemptTimeout)),
	)

	return download.Download(ctx, ipv4Endpoint, options...)
}

func (s *Scaleway) getMetadata(ctx context.Context) (*instance.Metadata, error) {
	metaConfigDl, err := downloadAlternating(ctx, ScalewayMetadataEndpoint, ScalewayMetadataEndpointIPv6)
	if err != nil {
		return nil, fmt.Errorf("error fetching metadata: %w", err)
	}

	var meta instance.Metadata
	if err = json.Unmarshal(metaConfigDl, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
