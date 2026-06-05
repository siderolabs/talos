// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

// LVMLogicalVolumeType describes the layout of an LVM logical volume.
type LVMLogicalVolumeType int

// LVM logical volume types.
//
//structprotogen:gen_enum
const (
	LVMLogicalVolumeTypeLinear LVMLogicalVolumeType = iota // linear
	LVMLogicalVolumeTypeRAID1                              // raid1
	LVMLogicalVolumeTypeRAID0                              // raid0
	LVMLogicalVolumeTypeRAID10                             // raid10
)
