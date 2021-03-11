// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package talos implements modern ADV which supports large size for the values and tags.
package talos

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv"
)

// Basic constants configuring the ADV.
const (
	Length     = 256 * 1024 // 256KiB
	DataLength = Length - 40
	Size       = 2 * Length // Redundancy
)

// Magic constants.
const (
	Magic1 = 0x5a4b3c2d
	Magic2 = 0xa5b4c3d2
)

// Tag is the key.
//
// We use a byte here for compatibility with syslinux, but format has space for uint32.
type Tag uint8

// Value stored for the tag.
type Value []byte

// ADV implements the Talos extended ADV.
//
// Layout (all in big-endian):
//   0x0000   4 bytes       magic1
//   0x0004   4 bytes       tag
//   0x0008   4 bytes       size
//   0x000c   (size) bytes  value
//   ... more tags
//  -0x0024   32 bytes      sha256 of the whole block with checksum set to zero
//  -0x0004   4 bytes       magic2
//
// Whole data structure is written twice for redundancy.
type ADV struct {
	Tags map[Tag]Value
}

// NewADV loads ADV from the block device.
func NewADV(r io.Reader) (*ADV, error) {
	a := &ADV{
		Tags: make(map[Tag]Value),
	}

	buf := make([]byte, Length)

	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	if err = a.Unmarshal(buf); err == nil {
		return a, nil
	}

	// try 2nd copy
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	return a, a.Unmarshal(buf)
}

// Unmarshal single copy from the serialized representation.
func (a *ADV) Unmarshal(buf []byte) error {
	magic1 := binary.BigEndian.Uint32(buf[:4])
	if magic1 != Magic1 {
		return fmt.Errorf("adv: unexpected magic %x, expecting %x", magic1, Magic1)
	}

	magic2 := binary.BigEndian.Uint32(buf[len(buf)-4:])
	if magic2 != Magic2 {
		return fmt.Errorf("adv: unexpected magic %x, expecting %x", magic2, Magic2)
	}

	checksum := append([]byte(nil), buf[len(buf)-36:len(buf)-4]...)

	copy(buf[len(buf)-36:len(buf)-4], make([]byte, 32))

	hash := sha256.New()
	hash.Write(buf)
	actualChecksum := hash.Sum(nil)

	if !bytes.Equal(checksum, actualChecksum) {
		return fmt.Errorf("adv: checksum mismatch: %x, expecting %x", checksum, actualChecksum)
	}

	data := buf[4 : len(buf)-36]

	for len(data) >= 8 {
		tag := binary.BigEndian.Uint32(data[:4])
		if tag == adv.End {
			break
		}

		size := binary.BigEndian.Uint32(data[4:8])

		if uint32(len(data)) < size+8 {
			return fmt.Errorf("adv: value goes beyond the end of the buffer: tag %d, size %d", tag, size)
		}

		value := data[8 : 8+size]

		a.Tags[Tag(tag)] = Value(value)

		data = data[8+size:]
	}

	return nil
}

// Marshal single copy of ADV.
func (a *ADV) Marshal() ([]byte, error) {
	buf := make([]byte, Length)

	binary.BigEndian.PutUint32(buf[0:4], Magic1)
	binary.BigEndian.PutUint32(buf[len(buf)-4:], Magic2)

	data := buf[4 : len(buf)-36]

	for tag, value := range a.Tags {
		if len(value)+8 > len(data) {
			return nil, fmt.Errorf("adv: overflow %d bytes", len(value)+8-len(data))
		}

		binary.BigEndian.PutUint32(data[0:4], uint32(tag))
		binary.BigEndian.PutUint32(data[4:8], uint32(len(value)))
		copy(data[8:8+len(value)], value)

		data = data[8+len(value):]
	}

	hash := sha256.New()
	hash.Write(buf)
	copy(buf[len(buf)-36:len(buf)-4], hash.Sum(nil))

	return buf, nil
}

// Bytes marshal full representation.
func (a *ADV) Bytes() ([]byte, error) {
	marshaled, err := a.Marshal()
	if err != nil {
		return nil, err
	}

	return append(marshaled, marshaled...), nil
}

// ReadTag to get tag value.
func (a *ADV) ReadTag(t uint8) (val string, ok bool) {
	b, ok := a.ReadTagBytes(t)

	val = string(b)

	return
}

// ReadTagBytes to get tag value.
func (a *ADV) ReadTagBytes(t uint8) (val []byte, ok bool) {
	val, ok = a.Tags[Tag(t)]

	return
}

// SetTag to set tag value.
func (a *ADV) SetTag(t uint8, val string) (ok bool) {
	return a.SetTagBytes(t, []byte(val))
}

// SetTagBytes to set tag value.
func (a *ADV) SetTagBytes(t uint8, val []byte) (ok bool) {
	size := 20 // magic + checksum

	for _, v := range a.Tags {
		size += len(v) + 8
	}

	oldVal := a.Tags[Tag(t)]

	size += len(Value(val)) - len(oldVal)

	if size > DataLength {
		return false
	}

	a.Tags[Tag(t)] = Value(val)

	return true
}

// DeleteTag to delete tag value.
func (a *ADV) DeleteTag(t uint8) (ok bool) {
	_, ok = a.Tags[Tag(t)]

	delete(a.Tags, Tag(t))

	return
}
