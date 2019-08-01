/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package lba provides a library for working with Logical Block Addresses.
package lba

import (
	"fmt"
)

// Range represents a range of Logical Block Addresses.
type Range struct {
	Start uint64
	End   uint64
}

// LogicalBlockAddresser represents Logical Block Addressing.
type LogicalBlockAddresser struct {
	PhysicalBlockSize int
	LogicalBlockSize  int
}

// Make returns a slice from a source slice in the the specified range inclusively.
func (lba *LogicalBlockAddresser) Make(size int) []byte {
	return make([]byte, lba.PhysicalBlockSize*size)
}

// Copy copies from src to dst in the specified range.
func (lba *LogicalBlockAddresser) Copy(dst []byte, src []byte, rng Range) (int, error) {
	size := uint64(lba.PhysicalBlockSize)
	n := copy(dst[size*rng.Start:size*rng.End], src)

	if n != len(src) {
		return -1, fmt.Errorf("expected to write %d elements, wrote %d", len(src), n)
	}

	return n, nil
}

// From returns a slice from a source slice in the the specified range inclusively.
func (lba *LogicalBlockAddresser) From(src []byte, rng Range) ([]byte, error) {
	size := uint64(lba.PhysicalBlockSize)
	if uint64(len(src)) < size+size*rng.End {
		return nil, fmt.Errorf("cannot read LBA range (start: %d, end %d), source too small", rng.Start, rng.End)
	}
	return src[size*rng.Start : size+size*rng.End], nil
}
