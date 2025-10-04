// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
)

// DiskImage configures Talos to boot from a disk image from the image factory.
type DiskImage struct{}

// Name implements the Preset interface.
func (DiskImage) Name() string { return "disk-image" }

// Description implements the Preset interface.
func (DiskImage) Description() string {
	return "Configure Talos to boot from a disk image from the image factory."
}

// ModifuOptions implements the Preset interface.
func (DiskImage) ModifuOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error {
	diskImageURL, err := url.JoinPath(presetOps.ImageFactoryURL.String(), "image", presetOps.SchematicID, cOps.TalosVersion, "metal-"+qOps.TargetArch)
	if presetOps.secureBoot {
		diskImageURL += secureBootSuffix
	}

	diskImageURL += ".raw.zst"

	if err != nil {
		return fmt.Errorf("failed to build an image factory disk-image url: %w", err)
	}

	qOps.NodeDiskImagePath = diskImageURL

	return nil
}
