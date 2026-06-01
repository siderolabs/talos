// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/pkg/machinery/platforms"
)

// PXE configures Talos to boot from via pxe from the Image Factory.
type PXE struct{}

// Name implements the Preset interface.
func (PXE) Name() string { return "pxe" }

// Description implements the Preset interface.
func (PXE) Description() string {
	return "Configure Talos to boot via PXE from the Image Factory."
}

// ModifyOptions implements the Preset interface.
func (PXE) ModifyOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error {
	pxeURL, err := url.JoinPath(presetOps.ImageFactoryURL.String(), "pxe", presetOps.SchematicID, cOps.TalosVersion,
		platforms.MetalPlatform().PXEScriptPath(qOps.TargetArch))
	if err != nil {
		return fmt.Errorf("failed to build an Image Factory pxe url: %w", err)
	}

	qOps.NodeIPXEBootScript = pxeURL

	return nil
}
