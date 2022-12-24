// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oracle

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/download"
)

// Ref: https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/gettingmetadata.htm
const (
	// OracleMetadataEndpoint is the local metadata endpoint for the hostname.
	OracleMetadataEndpoint = "http://169.254.169.254/opc/v2/instance/"
	// OracleUserDataEndpoint is the local metadata endpoint inside of Oracle Cloud.
	OracleUserDataEndpoint = "http://169.254.169.254/opc/v2/instance/metadata/user_data"
	// OracleNetworkEndpoint is the local network metadata endpoint inside of Oracle Cloud.
	OracleNetworkEndpoint = "http://169.254.169.254/opc/v2/vnics/"

	oracleResolverServer = "169.254.169.254"
	oracleTimeServer     = "169.254.169.254"
)

// MetadataConfig represents a metadata Oracle instance.
type MetadataConfig struct {
	Hostname           string `json:"hostname,omitempty"`
	ID                 string `json:"id,omitempty"`
	Region             string `json:"region,omitempty"`
	AvailabilityDomain string `json:"availabilityDomain,omitempty"`
	FaultDomain        string `json:"faultDomain,omitempty"`
	Shape              string `json:"shape,omitempty"`
}

func (o *Oracle) getMetadata(ctx context.Context) (*MetadataConfig, error) {
	metaConfigDl, err := download.Download(ctx, OracleMetadataEndpoint, //nolint:errcheck
		download.WithHeaders(map[string]string{"Authorization": "Bearer Oracle"}),
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		return nil, fmt.Errorf("error fetching metadata: %w", err)
	}

	var meta MetadataConfig
	if err = json.Unmarshal(metaConfigDl, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
