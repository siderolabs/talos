// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"

// Maintenance configures Talos to boot from a disk image from the Image Factory.
type Maintenance struct{}

// Name implements the Preset interface.
func (Maintenance) Name() string { return "maintenance" }

// Description implements the Preset interface.
func (Maintenance) Description() string {
	return "Skip applying machine configuration and leave the machines in maintenance mode. The machine configuration files are written to the working directory."
}

// ModifyOptions implements the Preset interface.
func (Maintenance) ModifyOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error {
	cOps.SkipInjectingConfig = true
	cOps.ApplyConfigEnabled = false

	return nil
}
