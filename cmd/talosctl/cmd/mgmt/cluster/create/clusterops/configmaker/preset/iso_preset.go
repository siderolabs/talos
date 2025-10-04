// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
)

// ISO configures Talos to boot from an iso from the image factory.
type ISO struct{}

// Name implements the Preset interface.
func (ISO) Name() string { return "iso" }

// Description implements the Preset interface.
func (ISO) Description() string {
	return "Configure Talos to boot from an ISO from the image factory."
}

// ModifuOptions implements the Preset interface.
func (ISO) ModifuOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error {
	isoURL, err := url.JoinPath(presetOps.ImageFactoryURL.String(), "image", presetOps.SchematicID, cOps.TalosVersion, "metal-"+qOps.TargetArch)
	if err != nil {
		return fmt.Errorf("failed to build an image factory iso url: %w", err)
	}

	if presetOps.secureBoot {
		isoURL += secureBootSuffix
	}

	isoURL += ".iso"

	qOps.NodeISOPath = isoURL

	return nil
}
