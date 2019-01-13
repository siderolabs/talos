/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package iso9660

import "bytes"

const (
	// Magic is the ISO 9660 magic signature.
	Magic = "CD001"
)

// SuperBlock represents the ISO 9660 super block.
type SuperBlock struct {
	FType           uint8
	ID              [5]uint8
	Version         uint8
	Flags           uint8
	SystemID        [32]uint8
	VolumeID        [32]uint8
	_               [8]uint8
	SpaceSize       [8]uint8
	EscapeSequences [8]uint8
	_               [222]uint8
	PublisherID     [128]uint8
	_               [128]uint8
	ApplicationID   [128]uint8
	_               [111]uint8
}

// Is implements the SuperBlocker interface.
func (sb *SuperBlock) Is() bool {
	trimmed := bytes.Trim(sb.ID[:], " ")
	return bytes.Equal(trimmed, []byte(Magic))
}

// Offset implements the SuperBlocker interface.
func (sb *SuperBlock) Offset() int64 {
	return 0x8000
}

// Type implements the SuperBlocker interface.
func (sb *SuperBlock) Type() string {
	return "iso9660"
}
