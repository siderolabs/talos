// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"

// SecureBoot configures Talos to boot from a disk image from the image factory.
type SecureBoot struct{}

// Name implements the Preset interface.
func (SecureBoot) Name() string { return "secureboot" }

// Description implements the Preset interface.
func (SecureBoot) Description() string {
	return "Configure Talos for secureboot."
}

// ModifuOptions implements the Preset interface.
func (SecureBoot) ModifuOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error {
	qOps.Tpm2Enabled = true
	qOps.DiskEncryptionKeyTypes = []string{"tpm"}
	qOps.EncryptEphemeralPartition = true
	qOps.EncryptStatePartition = true

	return nil
}
