// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package preset

import "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"

// ISOSecureBoot configures Talos to boot from a disk image from the Image Factory.
type ISOSecureBoot struct{}

// Name implements the Preset interface.
func (ISOSecureBoot) Name() string { return "iso-secureboot" }

// Description implements the Preset interface.
func (ISOSecureBoot) Description() string {
	return "Configure Talos for Secureboot via ISO. Only available on Linux hosts."
}

// ModifyOptions implements the Preset interface.
func (ISOSecureBoot) ModifyOptions(presetOps Options, cOps *clusterops.Common, qOps *clusterops.Qemu) error {
	isoURL, err := getISOURL(presetOps, cOps, qOps)
	if err != nil {
		return err
	}

	qOps.NodeISOPath = isoURL
	qOps.Tpm2Enabled = true
	qOps.DiskEncryptionKeyTypes = []string{"tpm"}
	qOps.EncryptEphemeralPartition = true
	qOps.EncryptStatePartition = true

	return nil
}
