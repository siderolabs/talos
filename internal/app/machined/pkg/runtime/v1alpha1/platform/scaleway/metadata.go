// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"

	"github.com/siderolabs/talos/pkg/download"
)

const (
	// ScalewayMetadataEndpoint is the local Scaleway endpoint.
	ScalewayMetadataEndpoint = "http://169.254.42.42/conf?format=json"
	// ScalewayUserDataEndpoint is the local Scaleway endpoint for the config.
	ScalewayUserDataEndpoint = "http://169.254.42.42/user_data/cloud-init"
)

func (u *Scaleway) getMetadata(ctx context.Context) (*instance.Metadata, error) {
	metaConfigDl, err := download.Download(ctx, ScalewayMetadataEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error fetching metadata: %w", err)
	}

	var meta instance.Metadata
	if err = json.Unmarshal(metaConfigDl, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
