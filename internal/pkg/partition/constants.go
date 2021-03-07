// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package partition

// Type in partition table.
type Type = string

// GPT partition types.
//
// TODO: should be moved into the blockdevice library.
const (
	EFISystemPartition  Type = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	BIOSBootPartition   Type = "21686148-6449-6E6F-744E-656564454649"
	LinuxFilesystemData Type = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
)

// FileSystemType is used to format partitions.
type FileSystemType = string

// Filesystem types.
const (
	FilesystemTypeNone FileSystemType = "none"
	FilesystemTypeXFS  FileSystemType = "xfs"
	FilesystemTypeVFAT FileSystemType = "vfat"
)

// Partition default sizes.
const (
	MiB = 1024 * 1024

	EFISize      = 100 * MiB
	BIOSGrubSize = 1 * MiB
	BootSize     = 300 * MiB
	MetaSize     = 1 * MiB
	StateSize    = 100 * MiB
)
