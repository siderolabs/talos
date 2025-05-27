// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// FilesystemType describes filesystem type.
type FilesystemType int

// Filesystem types.
//
//structprotogen:gen_enum
const (
	FilesystemTypeNone    FilesystemType = iota // none
	FilesystemTypeXFS                           // xfs
	FilesystemTypeVFAT                          // vfat
	FilesystemTypeEXT4                          // ext4
	FilesystemTypeISO9660                       // iso9660
	FilesystemTypeSwap                          // swap
)
