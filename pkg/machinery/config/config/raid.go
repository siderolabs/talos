// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/talos/pkg/machinery/cel"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// RAIDArrayConfig exposes a RAID (MD) array config document.
type RAIDArrayConfig interface {
	NamedDocument
	RAIDArrayConfigSignal()
	RAIDLevel() storageres.MDLevel
	RAIDMetadata() storageres.MDMetadata
	Provisioning() RAIDProvisioningConfig
}

// RAIDProvisioningConfig defines the interface to provision RAID arrays.
//
//nolint:iface
type RAIDProvisioningConfig interface {
	VolumeSelector() cel.Expression
}
