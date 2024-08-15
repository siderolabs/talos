// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cloudstack

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/download"
)

const (
	// CloudstackMetadataEndpoint is the local Cloudstack endpoint.
	CloudstackMetadataEndpoint = "http://data-server./latest/meta-data"
	// CloudstackUserDataEndpoint is the local Cloudstack endpoint for the config.
	CloudstackUserDataEndpoint = "http://data-server./latest/user-data"
)

// MetadataConfig represents a metadata Cloudstack instance.
type MetadataConfig struct {
	Hostname     string `json:"local-hostname,omitempty"`
	InstanceID   string `json:"instance-id,omitempty"`
	InstanceType string `json:"service-offering,omitempty"`
	PublicIPv4   string `json:"public-ipv4,omitempty"`
	Zone         string `json:"availability-zone,omitempty"`
}

/*
local-ipv4
public-hostname
vm-id
public-keys
cloud-identifier
hypervisor-host-name
*/

func (e *Cloudstack) getMetadata(ctx context.Context) (metadata *MetadataConfig, err error) {
	getMetadataKey := func(key string) (string, error) {
		res, metaerr := download.Download(ctx, fmt.Sprintf("%s/%s", CloudstackMetadataEndpoint, key),
			download.WithErrorOnNotFound(errors.ErrNoConfigSource),
			download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
		if metaerr != nil && !stderrors.Is(metaerr, errors.ErrNoConfigSource) {
			return "", fmt.Errorf("failed to fetch %q from IMDS: %w", key, metaerr)
		}

		return string(res), nil
	}

	metadata = &MetadataConfig{}

	if metadata.Hostname, err = getMetadataKey("local-hostname"); err != nil {
		return nil, err
	}

	if metadata.InstanceType, err = getMetadataKey("service-offering"); err != nil {
		return nil, err
	}

	if metadata.InstanceID, err = getMetadataKey("instance-id"); err != nil {
		return nil, err
	}

	if metadata.PublicIPv4, err = getMetadataKey("public-ipv4"); err != nil {
		return nil, err
	}

	if metadata.Zone, err = getMetadataKey("availability-zone"); err != nil {
		return nil, err
	}

	return metadata, nil
}
