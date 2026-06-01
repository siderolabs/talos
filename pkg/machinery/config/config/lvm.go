// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/talos/pkg/machinery/cel"
)

// LVMVolumeGroupConfig exposes an LVM volume group config document.
type LVMVolumeGroupConfig interface {
	NamedDocument
	LVMVolumeGroupConfigSignal()
	PhysicalVolumeSelector() cel.Expression
}
