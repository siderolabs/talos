// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package syslinux provides syslinux-compatible ADV data.
package syslinux

import (
	"encoding/binary"
	"io"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv"
)

const (
	// AdvSize is the total size.
	AdvSize = 512
	// AdvLen is the usable data size.
	AdvLen = AdvSize - 3*4
	// AdvMagic1 is the head signature.
	AdvMagic1 = uint32(0x5a2d2fa5)
	// AdvMagic2 is the total checksum.
	AdvMagic2 = uint32(0xa3041767)
	// AdvMagic3 is the tail signature.
	AdvMagic3 = uint32(0xdd28bf64)
)

// ADV represents the Syslinux Auxiliary Data Vector.
type ADV []byte

// NewADV returns the Auxiliary Data Vector.
func NewADV(r io.ReadSeeker) (adv ADV, err error) {
	_, err = r.Seek(-2*AdvSize, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	b := make([]byte, 2*AdvSize)

	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}

	adv = b

	return adv, nil
}

// ReadTag reads a tag in the ADV.
func (a ADV) ReadTag(t uint8) (val string, ok bool) {
	var b []byte

	b, ok = a.ReadTagBytes(t)
	val = string(b)

	return
}

// ReadTagBytes reads a tag in the ADV.
func (a ADV) ReadTagBytes(t uint8) (val []byte, ok bool) {
	// Header is in first 8 bytes.
	i := 8

	// End at tail plus two bytes required for successful next tag.
	for i < AdvSize-4-2 {
		tag := a[i]
		size := int(a[i+1])

		if tag == adv.End {
			break
		}

		if tag != t {
			// Jump to the next tag.
			i += 2 + size

			continue
		}

		length := int(a[i+1]) + i

		val = a[i+2 : length+2]

		ok = true

		break
	}

	return val, ok
}

// SetTag sets a tag in the ADV.
func (a ADV) SetTag(t uint8, val string) bool {
	return a.SetTagBytes(t, []byte(val))
}

// SetTagBytes sets a tag in the ADV.
func (a ADV) SetTagBytes(t uint8, val []byte) (ok bool) {
	if len(val) > 255 {
		return false
	}

	// Header is in first 8 bytes.
	i := 8

	// End at tail plus two bytes required for successful next tag.
	for i < AdvSize-4-2 {
		tag := a[i]
		size := int(a[i+1])

		if tag != adv.End {
			// Jump to the next tag.
			i += 2 + size

			continue
		}

		length := uint8(len(val))

		a[i] = t

		a[i+1] = length

		copy(a[i+2:uint8(i+2)+length], val)

		ok = true

		break
	}

	if ok {
		a.cleanup()
	}

	return ok
}

// DeleteTag deletes a tag in the ADV.
func (a ADV) DeleteTag(t uint8) (ok bool) {
	// Header is in first 8 bytes.
	i := 8

	// End at tail plus two bytes required for successful next tag.
	for i < AdvSize-4-2 {
		tag := a[i]
		size := int(a[i+1])

		if tag == adv.End {
			break
		}

		if tag != t {
			// Jump to the next tag.
			i += 2 + size

			continue
		}

		// Save the data after the tag that we will shift to the left by 2 + length
		// of the tag data.
		start := i + 2 + size

		end := a[AdvSize-4]

		data := make([]byte, len(a[start:end]))

		copy(data, a[start:end])

		// The total size we want to zero out is the length of all the remaining
		// data we saved above.
		length := 2 + len(data)

		// Zero each element to the right.
		for j := i; j < length; j++ {
			a[j] = 0
		}

		// Shift the data.
		copy(a[i:], data)

		ok = true

		break
	}

	if ok {
		a.cleanup()
	}

	return ok
}

// Bytes returns serialized contents of ADV.
func (a ADV) Bytes() ([]byte, error) {
	return a, nil
}

func (a ADV) cleanup() {
	a.head()

	a.total()

	a.tail()

	copy(a[AdvSize:], a[:AdvSize])
}

func (a ADV) head() {
	binary.LittleEndian.PutUint32(a[0:4], AdvMagic1)
}

func (a ADV) total() {
	csum := AdvMagic2
	for i := 8; i < AdvSize-4; i += 4 {
		csum -= binary.LittleEndian.Uint32(a[i : i+4])
	}

	binary.LittleEndian.PutUint32(a[4:8], csum)
}

func (a ADV) tail() {
	binary.LittleEndian.PutUint32(a[AdvSize-4:AdvSize], AdvMagic3)
}
