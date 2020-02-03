// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lba provides a library for working with Logical Block Addresses.
package lba

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
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
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat disk error: %w", err)
	}

	var psize uint64
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKPBSZGET, uintptr(unsafe.Pointer(&psize))); errno != 0 {
		if st.Mode().IsRegular() {
			// not a device, assume default block size
			psize = 512
		} else {
			return nil, errors.New("BLKPBSZGET failed")
		}
	}

	var lsize uint64
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKSSZGET, uintptr(unsafe.Pointer(&lsize))); errno != 0 {
		if st.Mode().IsRegular() {
			// not a device, assume default block size
			lsize = 512
		} else {
			return nil, errors.New("BLKSSZGET failed")
		}
	}

	lba = &LogicalBlockAddresser{
		PhysicalBlockSize: psize,
		LogicalBlockSize:  lsize,
	}

	return lba, nil
}

// Make returns a slice from a source slice in the the specified range inclusively.
func (lba *LogicalBlockAddresser) Make(size uint64) []byte {
	return make([]byte, lba.LogicalBlockSize*size)
}

// Copy copies from src to dst in the specified range.
func (lba *LogicalBlockAddresser) Copy(dst []byte, src []byte, rng Range) (int, error) {
	size := lba.LogicalBlockSize
	n := copy(dst[size*rng.Start:size*rng.End], src)

	if n != len(src) {
		return -1, fmt.Errorf("expected to write %d elements, wrote %d", len(src), n)
	}

	return n, nil
}

// From returns a slice from a source slice in the the specified range inclusively.
func (lba *LogicalBlockAddresser) From(src []byte, rng Range) ([]byte, error) {
	size := lba.LogicalBlockSize
	if uint64(len(src)) < size+size*rng.End {
		return nil, fmt.Errorf("cannot read LBA range (start: %d, end %d), source too small", rng.Start, rng.End)
	}

	return src[size*rng.Start : size+size*rng.End], nil
}
