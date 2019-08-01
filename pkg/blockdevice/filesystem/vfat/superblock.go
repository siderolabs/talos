/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package vfat

import (
	"bytes"
)

const (
	// Magic is the VFAT magic signature.
	Magic = "FAT32"
)

// SuperBlock represents the vfat super block.
type SuperBlock struct {
	Ignored      [3]uint8
	Sysid        [8]uint8
	SectorSize   [2]uint8
	ClusterSize  uint8
	Reserved     uint16
	Fats         uint8
	DirEntries   [2]uint8
	Sectors      [2]uint8
	Media        uint8
	FatLength    uint16
	SecsTrack    uint16
	Heads        uint16
	Hidden       uint32
	TotalSect    uint32
	Fat32Length  uint32
	Flags        uint16
	Version      [2]uint8
	RootCluster  uint32
	FsinfoSector uint16
	BackupBoot   uint16
	Reserved2    [6]uint16
	Unknown      [3]uint8
	Serno        [4]uint8
	Label        [11]uint8
	Magic        [8]uint8
	Dummy2       [0x1fe - 0x5a]uint8
	Pmagic       [2]uint8
}

// Is implements the SuperBlocker interface.
func (sb *SuperBlock) Is() bool {
	trimmed := bytes.Trim(sb.Magic[:], " ")
	return bytes.Equal(trimmed, []byte(Magic))
}

// Offset implements the SuperBlocker interface.
func (sb *SuperBlock) Offset() int64 {
	return 0x0
}

// Type implements the SuperBlocker interface.
func (sb *SuperBlock) Type() string {
	return "vfat"
}
