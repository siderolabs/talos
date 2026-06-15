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
	FilesystemTypeNone     FilesystemType = iota // none
	FilesystemTypeXFS                            // xfs
	FilesystemTypeVFAT                           // vfat
	FilesystemTypeEXT4                           // ext4
	FilesystemTypeISO9660                        // iso9660
	FilesystemTypeSwap                           // swapi
	FilesystemTypeVirtiofs                       // virtiofs
	FilesystemTypeBtrfs                          // btrfs
)

// SupportsTrim returns true if the filesystem supports discarding unused blocks
// (the FITRIM ioctl, equivalent of the fstrim(8) command).
func (t FilesystemType) SupportsTrim() bool {
	switch t {
	case FilesystemTypeXFS, FilesystemTypeEXT4, FilesystemTypeBtrfs:
		return true
	case FilesystemTypeNone, FilesystemTypeVFAT, FilesystemTypeISO9660, FilesystemTypeSwap, FilesystemTypeVirtiofs:
		return false
	default:
		return false
	}
}
