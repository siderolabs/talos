// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/talos/pkg/machinery/cel"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// LVMVolumeGroupConfig exposes an LVM volume group config document.
type LVMVolumeGroupConfig interface {
	NamedDocument
	LVMVolumeGroupConfigSignal()
	PhysicalVolumeSelector() cel.Expression
}

// LVMLogicalVolumeConfig exposes an LVM logical volume config document.
//
// Sizes are exposed as resolved primitives (not config/types/block.Size) to
// avoid an import cycle: config/types/block depends on this package.
type LVMLogicalVolumeConfig interface {
	NamedDocument
	LVMLogicalVolumeConfigSignal()
	// VolumeGroup returns the parent volume group name.
	VolumeGroup() string
	// Type returns the LV layout.
	Type() storageres.LVMLogicalVolumeType
	// Mirrors returns the mirror count for raid1/raid10 (defaulting to 1), or
	// 0 when not applicable.
	Mirrors() uint32
	// Stripes returns the stripe count for raid0/raid10, or 0 when unset
	// (resolved to all available PVs by the reconcile controller).
	Stripes() uint32
	// MaxSizeBytes returns the absolute LV size in bytes, or 0 when the size
	// is expressed as a percentage of the VG.
	MaxSizeBytes() uint64
	// MaxSizePercentVG returns the LV size as a percentage of the VG, or 0
	// when the size is absolute.
	MaxSizePercentVG() uint32
	// MinSizeBytes returns the minimum LV size in bytes, or 0 when unset.
	MinSizeBytes() uint64
}
