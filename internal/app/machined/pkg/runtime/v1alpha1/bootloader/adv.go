// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bootloader

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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

const (
	// AdvEnd is the noop tag.
	AdvEnd = iota
	// AdvBootonce is the bootonce tag.
	AdvBootonce
	// AdvMenusave is the menusave tag.
	AdvMenusave
	// AdvReserved1 is a reserved tag.
	AdvReserved1
	// AdvReserved2 is a reserved tag.
	AdvReserved2
	// AdvReserved3 is a reserved tag.
	AdvReserved3
	// AdvUpgrade is the upgrade tag.
	AdvUpgrade
)

// Meta represents the meta reader.
type Meta struct {
	*os.File
	ADV
}

// ADV represents the Syslinux Auxiliary Data Vector.
type ADV []byte

// NewMeta initializes and returns a `Meta`.
func NewMeta() (meta *Meta, err error) {
	var f *os.File

	f, err = probe.GetPartitionWithName(constants.MetaPartitionLabel)
	if err != nil {
		return nil, err
	}

	adv, err := NewADV(f)
	if err != nil {
		return nil, err
	}

	return &Meta{
		File: f,
		ADV:  adv,
	}, nil
}

func (m *Meta) Read(b []byte) (int, error) {
	return m.File.Read(b)
}

func (m *Meta) Write() (int, error) {
	offset, err := m.File.Seek(-2*AdvSize, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	n, err := m.File.WriteAt(m.ADV, offset)
	if err != nil {
		return n, err
	}

	if n != 2*AdvSize {
		return n, fmt.Errorf("expected to write %d bytes, wrote %d", AdvLen*2, n)
	}

	return n, nil
}

// Revert reverts the default bootloader label to the previous installation.
//
// nolint: gocyclo
func (m *Meta) Revert() (err error) {
	label, ok := m.ReadTag(AdvUpgrade)
	if !ok {
		return nil
	}

	if label == "" {
		m.DeleteTag(AdvUpgrade)

		if _, err = m.Write(); err != nil {
			return err
		}

		return nil
	}

	g := &grub.Grub{}

	if err = g.Default(label); err != nil {
		return err
	}

	m.DeleteTag(AdvUpgrade)

	if _, err = m.Write(); err != nil {
		return err
	}

	return nil
}

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
	// Header is in first 8 bytes.
	i := 8

	// End at tail plus two bytes required for successful next tag.
	for i < AdvSize-4-2 {
		tag := a[i]
		size := int(a[i+1])

		if tag == AdvEnd {
			break
		}

		if tag != t {
			// Jump to the next tag.
			i += 2 + size
			continue
		}

		len := int(a[i+1]) + i

		val = string(a[i+2 : len+2])

		ok = true

		break
	}

	return val, ok
}

// SetTag sets a tag in the ADV.
func (a ADV) SetTag(t uint8, val string) (ok bool) {
	b := []byte(val)

	if len(b) > 255 {
		return false
	}

	// Header is in first 8 bytes.
	i := 8

	// End at tail plus two bytes required for successful next tag.
	for i < AdvSize-4-2 {
		tag := a[i]
		size := int(a[i+1])

		if tag != AdvEnd {
			// Jump to the next tag.
			i += 2 + size
			continue
		}

		length := uint8(len(b))

		a[i] = t

		a[i+1] = length

		copy(a[i+2:uint8(i+2)+length], b)

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

		if tag == AdvEnd {
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
