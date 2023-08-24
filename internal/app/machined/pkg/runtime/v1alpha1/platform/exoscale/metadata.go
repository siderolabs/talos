// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package exoscale

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/download"
)

const (
	// ExoscaleMetadataEndpoint is the local Exoscale endpoint.
	ExoscaleMetadataEndpoint = "http://169.254.169.254/1.0/meta-data"
	// ExoscaleUserDataEndpoint is the local Exoscale endpoint for the config.
	ExoscaleUserDataEndpoint = "http://169.254.169.254/1.0/user-data"
)

// MetadataConfig represents a metadata Exoscale instance.
type MetadataConfig struct {
	Hostname     string `json:"local-hostname,omitempty"`
	InstanceID   string `json:"instance-id,omitempty"`
	InstanceType string `json:"service-offering,omitempty"`
	PublicIPv4   string `json:"public-ipv4,omitempty"`
	Zone         string `json:"availability-zone,omitempty"`
}

func (e *Exoscale) getMetadata(ctx context.Context) (metadata *MetadataConfig, err error) {
	getMetadataKey := func(key string) (string, error) {
		res, metaerr := download.Download(ctx, fmt.Sprintf("%s/%s", ExoscaleMetadataEndpoint, key),
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
