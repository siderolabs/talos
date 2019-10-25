// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package xfs

const (
	// Magic is the XFS magic number.
	Magic = 0x58465342
)

// SuperBlock represents the xfs super block.
type SuperBlock struct {
	Magic      uint32
	Blocksize  uint32
	Dblocks    uint64
	Rblocks    uint64
	Rextents   uint64
	UUID       [16]uint8
	Logstart   uint64
	Rootino    uint64
	Rbmino     uint64
	Rsumino    uint64
	Rextsize   uint32
	Agblocks   uint32
	Agcount    uint32
	Rbmblocks  uint32
	Logblocks  uint32
	Versionnum uint16
	Sectsize   uint16
	Inodesize  uint16
	Inopblock  uint16
	Fname      [12]uint8
	Blocklog   uint8
	Sectlog    uint8
	Inodelog   uint8
	Inopblog   uint8
	Agblklog   uint8
	Rextslog   uint8
	Inprogress uint8
	ImaxPct    uint8
	Icount     uint64
	Ifree      uint64
	Fdblocks   uint64
	Frextents  uint64
}

// Is implements the SuperBlocker interface.
func (sb *SuperBlock) Is() bool {
	return sb.Magic == Magic
}

// Offset implements the SuperBlocker interface.
func (sb *SuperBlock) Offset() int64 {
	return 0x0
}

// Type implements the SuperBlocker interface.
func (sb *SuperBlock) Type() string {
	return "xfs"
}
