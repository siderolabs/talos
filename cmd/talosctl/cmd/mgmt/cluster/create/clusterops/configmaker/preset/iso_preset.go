// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import (
	"net/url"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/pkg/machinery/platforms"
)

// ISO configures Talos to boot from an iso from the image factory.
type ISO struct{}

// Name implements the Preset interface.
func (ISO) Name() string { return "iso" }

// Description implements the Preset interface.
func (ISO) Description() string {
	return "Configure Talos to boot from an ISO from the image factory."
}

// ModifyOptions implements the Preset interface.
func (ISO) ModifyOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error {
	isoURL, err := getISOURL(presetOps, cOps, qOps)
	if err != nil {
		return err
	}

	qOps.NodeISOPath = isoURL

	return nil
}

func getISOURL(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) (string, error) {
	if presetOps.secureBoot {
		return url.JoinPath(presetOps.ImageFactoryURL.String(), "image", presetOps.SchematicID, cOps.TalosVersion,
			platforms.MetalPlatform().SecureBootISOPath(qOps.TargetArch))
	}

	return url.JoinPath(presetOps.ImageFactoryURL.String(), "image", presetOps.SchematicID, cOps.TalosVersion,
		platforms.MetalPlatform().ISOPath(qOps.TargetArch))
}
