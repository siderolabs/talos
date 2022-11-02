// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vultr

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vultr/metadata"

	"github.com/siderolabs/talos/pkg/download"
)

const (
	// VultrMetadataEndpoint is the local Vultr endpoint fot the instance metadata.
	VultrMetadataEndpoint = "http://169.254.169.254/v1.json"
	// VultrExternalIPEndpoint is the local Vultr endpoint for the external IP.
	VultrExternalIPEndpoint = "http://169.254.169.254/latest/meta-data/public-ipv4"
	// VultrUserDataEndpoint is the local Vultr endpoint for the config.
	VultrUserDataEndpoint = "http://169.254.169.254/latest/user-data"
)

func (g *Vultr) getMetadata(ctx context.Context) (*metadata.MetaData, error) {
	metaConfigDl, err := download.Download(ctx, VultrMetadataEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error fetching metadata: %w", err)
	}

	var meta metadata.MetaData
	if err = json.Unmarshal(metaConfigDl, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
