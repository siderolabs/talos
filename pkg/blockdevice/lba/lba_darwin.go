// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lba provides a library for working with Logical Block Addresses.
package lba

import (
	"fmt"
	"os"
)

// Range represents a range of Logical Block Addresses.
type Range struct {
	Start uint64
	End   uint64
}

// LogicalBlockAddresser represents Logical Block Addressing.
type LogicalBlockAddresser struct {
	PhysicalBlockSize uint64
	LogicalBlockSize  uint64
}

// New initializes and returns a LogicalBlockAddresser.
func New(f *os.File) (lba *LogicalBlockAddresser, err error) {
	return nil, fmt.Errorf("not implemented")
}

// Make returns a slice from a source slice in the the specified range inclusively.
func (lba *LogicalBlockAddresser) Make(size uint64) []byte {
	return nil
}

// Copy copies from src to dst in the specified range.
func (lba *LogicalBlockAddresser) Copy(dst, src []byte, rng Range) (int, error) {
	return 0, fmt.Errorf("not implemented")
}

// From returns a slice from a source slice in the the specified range inclusively.
func (lba *LogicalBlockAddresser) From(src []byte, rng Range) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
