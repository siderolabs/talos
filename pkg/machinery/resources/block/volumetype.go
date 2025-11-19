// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// VolumeType describes volume type.
type VolumeType int

// Volume types.
//
//structprotogen:gen_enum
const (
	VolumeTypePartition VolumeType = iota // partition
	VolumeTypeDisk                        // disk
	VolumeTypeTmpfs                       // tmpfs
	VolumeTypeDirectory                   // directory
	VolumeTypeSymlink                     // symlink
	VolumeTypeOverlay                     // overlay
	VolumeTypeExternal                    // external
)
