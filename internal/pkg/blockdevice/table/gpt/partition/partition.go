/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package partition provides a library for working with GPT partitions.
package partition

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
	"github.com/talos-systems/talos/internal/pkg/serde"
	"golang.org/x/text/encoding/unicode"
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

	IsNew     bool
	IsResized bool
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
	return int64(prt.LastLBA)
}

// No returns the partition's number.
func (prt *Partition) No() int32 {
	return prt.Number
}

// Fields implements the serder.Serde interface.
func (prt *Partition) Fields() []*serde.Field {
	return []*serde.Field{
		// 16 bytes Partition type GUID
		{
			Offset: 0,
			Length: 16,
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				return prt.Type.MarshalBinary()
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				guid, err := uuid.FromBytes(contents)
				if err != nil {
					return fmt.Errorf("invalid GUUID: %v", err)
				}

				// TODO: Provide a method for getting the human readable name of the type.
				// See https://en.wikipedia.org/wiki/GUID_Partition_Table.
				prt.Type = guid

				return nil
			},
		},
		// 16 bytes Unique partition GUID
		{
			Offset: 16,
			Length: 16,
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				return prt.ID.MarshalBinary()
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				guid, err := uuid.FromBytes(contents)
				if err != nil {
					return fmt.Errorf("invalid GUUID: %v", err)
				}

				prt.ID = guid

				return nil
			},
		},
		// 8 bytes First LBA (little endian)
		{
			Offset: 32,
			Length: 8,
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, prt.FirstLBA)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				prt.FirstLBA = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 8 bytes Last LBA (inclusive, usually odd)
		{
			Offset: 40,
			Length: 8,
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, prt.LastLBA)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				prt.LastLBA = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 8 bytes Attribute flags (e.g. bit 60 denotes read-only)
		{
			Offset: 48,
			Length: 8,
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, prt.Flags)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				prt.Flags = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 72 bytes Partition name (36 UTF-16LE code units)
		{
			Offset: 56,
			Length: 72,
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
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
			SerializerFunc: func(contents []byte, opts interface{}) error {
				utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
				decoded, err := utf16.NewDecoder().Bytes(contents)
				if err != nil {
					return err
				}

				prt.Name = string(bytes.Trim(decoded, "\x00"))

				return nil
			},
		},
		{
			Offset: 72,
			Length: 56,
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				copy(data, prt.TrailingBytes)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				prt.TrailingBytes = contents

				return nil
			},
		},
	}
}
