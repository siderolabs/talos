/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package header provides a library for working with GPT headers.
package header

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"

	"github.com/google/uuid"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/lba"
	"github.com/talos-systems/talos/internal/pkg/serde"
)

const (
	// HeaderSize is the GUID partition table header size in bytes.
	HeaderSize = 92
)

// Header represents a GUID partition table.
type Header struct {
	data  []byte
	array []byte

	Signature                string    // 0
	Revision                 uint32    // 8
	Size                     uint32    // 12
	CRC                      uint32    // 16
	Reserved                 uint32    // 20
	CurrentLBA               uint64    // 24
	BackupLBA                uint64    // 32
	FirstUsableLBA           uint64    // 40
	LastUsableLBA            uint64    // 48
	GUUID                    uuid.UUID // 56
	PartitionEntriesStartLBA uint64    // 72
	NumberOfPartitionEntries uint32    // 80
	PartitionEntrySize       uint32    // 84
	PartitionsArrayCRC       uint32    // 88
	TrailingBytes            []byte    // 92

	*lba.LogicalBlockAddresser
}

// NewHeader inializes and returns a GUID partition table header.
func NewHeader(data []byte, lba *lba.LogicalBlockAddresser) *Header {
	return &Header{
		data:                  data,
		LogicalBlockAddresser: lba,
	}
}

// Bytes implements the table.Header interface.
func (hdr *Header) Bytes() []byte {
	return hdr.data
}

// ArrayBytes returns the GUID partition table partitions entries array as a byte slice.
func (hdr *Header) ArrayBytes() []byte {
	return hdr.array
}

// Fields impements the serde.Serde interface.
// nolint: gocyclo
func (hdr *Header) Fields() []*serde.Field {
	return []*serde.Field{
		// 8 bytes Signature ("EFI PART", 45h 46h 49h 20h 50h 41h 52h 54h or 0x5452415020494645ULL on little-endian machines)
		{
			Offset: 0,
			Length: 8,
			// Contents: []byte{0x45, 0x46, 0x49, 0x20, 0x50, 0x41, 0x52, 0x54},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				return []byte{0x45, 0x46, 0x49, 0x20, 0x50, 0x41, 0x52, 0x54}, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				signature := string(contents)
				if signature != "EFI PART" {
					return fmt.Errorf("expected signature of \"EFI PART\", got %q", signature)
				}

				hdr.Signature = string(contents)

				return nil
			},
		},
		// 4 bytes Revision (for GPT version 1.0 (through at least UEFI version 2.7 (May 2017)), the value is 00h 00h 01h 00h)
		{
			Offset: 8,
			Length: 4,
			// Contents: []byte{0x00, 0x00, 0x01, 0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint32(data, hdr.Revision)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				expected := []byte{0x00, 0x00, 0x01, 0x00}
				if !bytes.Equal(contents, expected) {
					return fmt.Errorf("expected revision of %v, got %v", expected, contents)
				}

				hdr.Revision = binary.LittleEndian.Uint32(contents)

				return nil
			},
		},
		// 4 bytes Header size in little endian (in bytes, usually 5Ch 00h 00h 00h or 92 bytes)
		{
			Offset: 12,
			Length: 4,
			// Contents: []byte{0x5c, 0x00, 0x00, 0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint32(data, hdr.Size)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				hdr.Size = binary.LittleEndian.Uint32(contents)
				if hdr.Size != HeaderSize {
					return fmt.Errorf("expected GPT header size of %d, got %d", HeaderSize, hdr.Size)
				}

				return nil
			},
		},
		// 4 bytes Reserved; must be zero
		{
			Offset: 20,
			Length: 4,
			// Contents: []byte{0x00, 0x00, 0x00, 0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				return []byte{0x00, 0x00, 0x00, 0x00}, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				expected := []byte{0x00, 0x00, 0x00, 0x00}
				if !bytes.Equal(contents, expected) {
					return fmt.Errorf("expected reserved field to be %v, got %v", expected, contents)
				}

				hdr.Reserved = binary.LittleEndian.Uint32(contents)

				return nil
			},
		},
		// 8 bytes Current LBA (location of this header copy)
		// nolint: dupl
		{
			Offset: 24,
			Length: 8,
			// Contents: []byte{0x00, 0x00, 0x00, 0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				o, ok := opts.(*Options)
				if !ok {
					return nil, fmt.Errorf("option is not a GPT header option")
				}
				if o.Primary {
					binary.LittleEndian.PutUint64(data, hdr.CurrentLBA)
				} else {
					binary.LittleEndian.PutUint64(data, hdr.BackupLBA)
				}

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				hdr.CurrentLBA = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 8 bytes Backup LBA (location of the other header copy)
		// nolint: dupl
		{
			Offset: 32,
			Length: 8,
			// Contents: []byte{0x00, 0x00, 0x00, 0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				o, ok := opts.(*Options)
				if !ok {
					return nil, fmt.Errorf("option is not a GPT header option")
				}
				if o.Primary {
					binary.LittleEndian.PutUint64(data, hdr.BackupLBA)

				} else {
					binary.LittleEndian.PutUint64(data, hdr.CurrentLBA)
				}

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				hdr.BackupLBA = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 8 bytes First usable LBA for partitions (primary partition table last LBA + 1)
		{
			Offset: 40,
			Length: 8,
			// Contents: []byte{0x00, 0x00, 0x00, 0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, hdr.FirstUsableLBA)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				hdr.FirstUsableLBA = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 8 bytes Last usable LBA (secondary partition table first LBA - 1)
		{
			Offset: 48,
			Length: 8,
			// Contents: []byte{0x00, 0x00, 0x00, 0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, hdr.LastUsableLBA)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				hdr.LastUsableLBA = binary.LittleEndian.Uint64(contents)

				return nil
			},
		},
		// 16 bytes Disk GUID (also referred as UUID on UNIXes)
		{
			Offset: 56,
			Length: 16,
			// Contents: []byte{0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				return hdr.GUUID.MarshalBinary()
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				guid, err := uuid.FromBytes(contents)
				if err != nil {
					return fmt.Errorf("invalid GUUID: %v", err)
				}

				hdr.GUUID = guid

				return nil
			},
		},
		// 8 bytes Starting LBA of array of partition entries (always 2 in primary copy)
		{
			Offset: 72,
			Length: 8,
			// Contents: []byte{0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint64(data, hdr.PartitionEntriesStartLBA)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				// TODO: Should we verify it is 2 in the case of primary?
				o, ok := opts.(*Options)
				if !ok {
					return fmt.Errorf("option is not a GPT header option")
				}
				hdr.PartitionEntriesStartLBA = binary.LittleEndian.Uint64(contents)
				array, err := hdr.From(o.Table, lba.Range{Start: hdr.PartitionEntriesStartLBA, End: uint64(33)})
				if err != nil {
					return fmt.Errorf("failed to read starting LBA from header: %v", err)
				}

				hdr.array = array

				return nil
			},
		},
		// 4 bytes Number of partition entries in array
		{
			Offset: 80,
			Length: 4,
			// Contents: []byte{0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint32(data, hdr.NumberOfPartitionEntries)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				hdr.NumberOfPartitionEntries = binary.LittleEndian.Uint32(contents)

				return nil
			},
		},
		// 4 bytes Size of a single partition entry (usually 80h or 128)
		{
			Offset: 84,
			Length: 4,
			// Contents: []byte{0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, length)
				binary.LittleEndian.PutUint32(data, hdr.PartitionEntrySize)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				length := binary.LittleEndian.Uint32(contents)
				// This field should be set to a value of: 128 x 2n where n is an integer greater than or equal to zero.
				if length%128 != 0 {
					return fmt.Errorf("expected partition entry size to be a multiple of %d, got %d", 128, length)
				}

				hdr.PartitionEntrySize = binary.LittleEndian.Uint32(contents)

				return nil
			},
		},
		// 4 bytes CRC32/zlib of partition array in little endian
		{
			Offset: 88,
			Length: 4,
			// Contents: []byte{0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				o, ok := opts.(*Options)
				if !ok {
					return nil, fmt.Errorf("option is not a GPT header option")
				}
				expected := hdr.NumberOfPartitionEntries * hdr.PartitionEntrySize
				if len(o.Array) != int(expected) {
					return nil, fmt.Errorf("expected array length of %d, got %d", expected, len(o.Array))
				}
				crc := crc32.ChecksumIEEE(o.Array)
				data := make([]byte, length)
				binary.LittleEndian.PutUint32(data, crc)

				// We should update this here and now to ensure that the field is still valid.
				hdr.PartitionsArrayCRC = crc

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				crc := binary.LittleEndian.Uint32(contents)
				// Note that hdr.array is expected to be set in offset 72 DeserializerFunc.
				checksum := crc32.ChecksumIEEE(hdr.array)
				if crc != checksum {
					return fmt.Errorf("expected partition checksum of %v, got %v", checksum, crc)
				}

				hdr.PartitionsArrayCRC = crc

				return nil
			},
		},
		// Reserved; must be zeroes for the rest of the block (420 bytes for a sector size of 512 bytes; but can be more with larger sector sizes)
		{
			Offset: HeaderSize,
			Length: 420,
			// Contents: []byte{0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				data := make([]byte, 420)

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				expected := make([]byte, 420)
				if !bytes.Equal(contents, expected) {
					return fmt.Errorf("expected %d trailing bytes of zeroes", 420)
				}

				hdr.TrailingBytes = contents

				return nil
			},
		},
		// 4 bytes CRC32/zlib of header (offset +0 up to header size) in little endian, with this field zeroed during calculation
		{
			Offset: 16,
			Length: 4,
			// Contents: []byte{0x00, 0x00, 0x00, 0x00},
			DeserializerFunc: func(offset, length uint32, new []byte, opts interface{}) ([]byte, error) {
				// Copy the header into a temporary slice and to avoid modifying the original.
				header := make([]byte, HeaderSize)
				copy(header, new)
				// Zero the CRC field during the calculation.
				copy(header[16:20], []byte{0x00, 0x00, 0x00, 0x00})

				crc := crc32.ChecksumIEEE(header)
				data := make([]byte, length)
				binary.LittleEndian.PutUint32(data, crc)

				// We should update this here and now to ensure that the field is still valid.
				hdr.CRC = crc

				return data, nil
			},
			SerializerFunc: func(contents []byte, opts interface{}) error {
				crc := binary.LittleEndian.Uint32(contents)

				// Copy the header into a temporary slice and to avoid modifying the original.
				header := make([]byte, HeaderSize)
				copy(header, hdr.data)
				// Zero the CRC field during the calculation.
				copy(header[16:20], []byte{0x00, 0x00, 0x00, 0x00})

				checksum := crc32.ChecksumIEEE(header)
				if crc != checksum {
					return fmt.Errorf("expected header checksum of %d, got %d", crc, checksum)
				}

				hdr.CRC = crc

				return nil
			},
		},
	}
}
