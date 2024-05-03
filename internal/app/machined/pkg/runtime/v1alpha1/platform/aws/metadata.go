// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// MetadataConfig represents a metadata AWS instance.
type MetadataConfig struct {
	Hostname          string `json:"hostname,omitempty"`
	InstanceID        string `json:"instance-id,omitempty"`
	InstanceType      string `json:"instance-type,omitempty"`
	InstanceLifeCycle string `json:"instance-life-cycle,omitempty"`
	PublicIPv4        string `json:"public-ipv4,omitempty"`
	PublicIPv6        string `json:"ipv6,omitempty"`
	InternalDNS       string `json:"local-hostname,omitempty"`
	ExternalDNS       string `json:"public-hostname,omitempty"`
	Region            string `json:"region,omitempty"`
	Zone              string `json:"zone,omitempty"`
}

//nolint:gocyclo
func (a *AWS) getMetadata(ctx context.Context) (*MetadataConfig, error) {
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
	getMetadataKey := func(key string) (string, error) {
		resp, err := a.metadataClient.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: key,
		})
		if err != nil {
			if isNotFoundError(err) {
				return "", nil
			}

			return "", fmt.Errorf("failed to fetch %q from IMDS: %w", key, err)
		}

		defer resp.Content.Close() //nolint:errcheck

		v, err := io.ReadAll(resp.Content)

		return string(v), err
	}

	var (
		metadata MetadataConfig
		err      error
	)

	if metadata.Hostname, err = getMetadataKey("hostname"); err != nil {
		return nil, err
	}

	if metadata.InstanceType, err = getMetadataKey("instance-type"); err != nil {
		return nil, err
	}

	if metadata.InstanceLifeCycle, err = getMetadataKey("instance-life-cycle"); err != nil {
		return nil, err
	}

	if metadata.InstanceID, err = getMetadataKey("instance-id"); err != nil {
		return nil, err
	}

	if metadata.PublicIPv4, err = getMetadataKey("public-ipv4"); err != nil {
		return nil, err
	}

	if metadata.PublicIPv6, err = getMetadataKey("ipv6"); err != nil {
		return nil, err
	}

	if metadata.InternalDNS, err = getMetadataKey("local-hostname"); err != nil {
		return nil, err
	}

	if metadata.ExternalDNS, err = getMetadataKey("public-hostname"); err != nil {
		return nil, err
	}

	if metadata.Region, err = getMetadataKey("placement/region"); err != nil {
		return nil, err
	}

	if metadata.Zone, err = getMetadataKey("placement/availability-zone"); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func isNotFoundError(err error) bool {
	var awsErr *smithyhttp.ResponseError

	if errors.As(err, &awsErr) {
		return awsErr.HTTPStatusCode() == http.StatusNotFound
	}

	return false
}
