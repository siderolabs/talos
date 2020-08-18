// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package partition provides a library for working with GPT partitions.
package partition

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/text/encoding/unicode"

	"github.com/talos-systems/talos/pkg/endianness"
	"github.com/talos-systems/talos/pkg/serde"
)

// Partition represents a partition entry in a GUID partition table.
type Partition struct {
	data []byte

	Type          uuid.UUID // 0
	ID            uuid.UUID // 16
	FirstLBA      uint64    // 32
	LastLBA       uint64    // 40
	Flags         uint64    // 48
	Name          string    // 56
	TrailingBytes []byte    // 128

	Number int32
}

// NewPartition initializes and returns a new partition.
func NewPartition(data []byte) *Partition {
	return &Partition{
		data: data,
	}
}

// Bytes returns the partition as a byte slice.
func (prt *Partition) Bytes() []byte {
	return prt.data
}

// Start returns the partition's starting LBA..
func (prt *Partition) Start() int64 {
	return int64(prt.FirstLBA)
}

// Length returns the partition's length in LBA.
func (prt *Partition) Length() int64 {
	// TODO(andrewrynhard): For reasons I don't understand right now, we need
	// to add 1 in order to align with what partx thinks is the length of the
	// partition.
	return int64(prt.LastLBA - prt.FirstLBA + 1)
}

// No returns the partition's number.
func (prt *Partition) No() int32 {
	return prt.Number
}

// Fields implements the serder.Serde interface.
func (prt *Partition) Fields() []*serde.Field {
	return []*serde.Field{
		// 16 bytes Partition type GUID
		// nolint: dupl
		{
			Offset: 0,
			Length: 16,
			SerializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				b, err := prt.Type.MarshalBinary()
				if err != nil {
					return nil, err
				}

				return endianness.ToMiddleEndian(b)
			},
			DeserializerFunc: func(contents []byte, opts interface{}) error {
				u, err := endianness.FromMiddleEndian(contents)
				if err != nil {
					return err
				}

				guid, err := uuid.FromBytes(u)
				if err != nil {
					return fmt.Errorf("invalid GUUID: %w", err)
				}

				// TODO: Provide a method for getting the human readable name of the type.
				// See https://en.wikipedia.org/wiki/GUID_Partition_Table.
				prt.Type = guid

				return nil
			},
		},
		// 16 bytes Unique partition GUID
		// nolint: dupl
		{
			Offset: 16,
			Length: 16,
			SerializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				b, err := prt.ID.MarshalBinary()
				if err != nil {
					return nil, err
				}

				return endianness.ToMiddleEndian(b)
			},
			DeserializerFunc: func(contents []byte, opts interface{}) error {
				u, err := endianness.FromMiddleEndian(contents)
				if err != nil {
					return err
				}

				guid, err := uuid.FromBytes(u)
				if err != nil {
					return fmt.Errorf("invalid GUUID: %w", err)
				}

				prt.ID = guid

				return nil
			},
		},
		// 8 bytes First LBA (little endian)
		{
			Offset: 32,
			Length: 8,
			SerializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, prt.FirstLBA)

				return data, nil
			},
			DeserializerFunc: func(contents []byte, opts interface{}) error {
				prt.FirstLBA = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 8 bytes Last LBA (inclusive, usually odd)
		{
			Offset: 40,
			Length: 8,
			SerializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, prt.LastLBA)

				return data, nil
			},
			DeserializerFunc: func(contents []byte, opts interface{}) error {
				prt.LastLBA = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 8 bytes Attribute flags (e.g. bit 60 denotes read-only)
		// Known attributes are:
		//   0: system partition
		//   1: hide from EFI
		//   2: legacy BIOS bootable
		//   60: read-only
		//   62: hidden
		//   63: do not automount
		{
			Offset: 48,
			Length: 8,
			SerializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, prt.Flags)

				return data, nil
			},
			DeserializerFunc: func(contents []byte, opts interface{}) error {
				prt.Flags = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 72 bytes Partition name (36 UTF-16LE code units)
		{
			Offset: 56,
			Length: 72,
			SerializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
				name, err := utf16.NewEncoder().Bytes([]byte(prt.Name))
				if err != nil {
					return nil, err
				}
				// TODO: Should we error if the name exceeds 72 bytes?
				data := make([]byte, 72)
				copy(data, name)

				return data, nil
			},
			DeserializerFunc: func(contents []byte, opts interface{}) error {
				utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
				decoded, err := utf16.NewDecoder().Bytes(contents)
				if err != nil {
					return err
				}

				prt.Name = string(bytes.Trim(decoded, "\x00"))

				return nil
			},
		},
	}
}
