// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

package efivarfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/pkg/msguid"
)

// DevicePath represents a path consisting of one or more elements to an
// entity implementing an EFI protocol. It's very broadly used inside EFI
// for representing all sorts of abstract paths. In the context of this
// package it is used to represent paths to EFI loaders.
// See https://uefi.org/specs/UEFI/2.10/10_Protocols_Device_Path_Protocol.html
// for more information.
type DevicePath []DevicePathElem

// DevicePathElem is a common interface for all UEFI device path elements.
type DevicePathElem interface {
	typ() uint8
	subType() uint8
	data() ([]byte, error)
}

type pathElemUnmarshalFunc func([]byte) (DevicePathElem, error)

// PartitionMBR matches a drive or partition formatted with legacy MBR
// (Master Boot Record).
type PartitionMBR struct {
	// DiskSignature contains a 4-byte signature identifying the drive, located
	// just after the 440 bytes of boot sector loading code.
	// Note that since MBR does not have per-partition signatures, this is
	// combined with PartitionNumber to select a partition.
	DiskSignature [4]byte
}

func (p PartitionMBR) partitionSignature() (sig [16]byte) {
	copy(sig[:4], p.DiskSignature[:])

	return sig
}

func (p PartitionMBR) partitionFormat() uint8 {
	return 0x01
}

func (p PartitionMBR) signatureType() uint8 {
	return 0x01
}

// PartitionGPT matches a partition on a drive formatted with GPT.
type PartitionGPT struct {
	// UUID of the partition to be matched. Conversion into mixed-endian format
	// is taken care of, a standard big-endian UUID can be put in here.
	PartitionUUID uuid.UUID
}

func (p PartitionGPT) partitionSignature() [16]byte {
	return msguid.From(p.PartitionUUID)
}

func (p PartitionGPT) partitionFormat() uint8 {
	return 0x02
}

func (p PartitionGPT) signatureType() uint8 {
	return 0x02
}

// PartitionUnknown is being used to represent unknown partitioning schemas or
// combinations of PartitionFormat/SignatureType. It contains raw uninterpreted
// data.
type PartitionUnknown struct {
	PartitionSignature [16]byte
	PartitionFormat    uint8
	SignatureType      uint8
}

func (p PartitionUnknown) partitionSignature() [16]byte {
	return p.PartitionSignature
}

func (p PartitionUnknown) partitionFormat() uint8 {
	return p.PartitionFormat
}

func (p PartitionUnknown) signatureType() uint8 {
	return p.SignatureType
}

// PartitionMatch is an interface that defines methods for matching
// partitions on drives or filepaths.
type PartitionMatch interface {
	partitionSignature() [16]byte
	partitionFormat() uint8
	signatureType() uint8
}

// HardDrivePath matches whole drives or partitions on GPT/MBR formatted
// drives.
type HardDrivePath struct {
	// Partition number, starting at 1. If zero or unset, the whole drive is
	// selected.
	PartitionNumber uint32
	// Block address at which the partition starts. Not used for matching
	// partitions in EDK2.
	PartitionStartBlock uint64
	// Number of blocks occupied by the partition starting from the
	// PartitionStartBlock. Not used for matching partitions in EDK2.
	PartitionSizeBlocks uint64
	// PartitionMatch is used to match drive or partition signatures.
	// Use PartitionMBR and PartitionGPT types here.
	PartitionMatch PartitionMatch
}

func (h *HardDrivePath) typ() uint8 {
	return 4
}

func (h *HardDrivePath) subType() uint8 {
	return 1
}

func (h *HardDrivePath) data() ([]byte, error) {
	out := make([]byte, 38)
	le := binary.LittleEndian
	le.PutUint32(out[0:4], h.PartitionNumber)
	le.PutUint64(out[4:12], h.PartitionStartBlock)
	le.PutUint64(out[12:20], h.PartitionSizeBlocks)

	if h.PartitionMatch == nil {
		return nil, errors.New("PartitionMatch needs to be set")
	}

	sig := h.PartitionMatch.partitionSignature()
	copy(out[20:36], sig[:])
	out[36] = h.PartitionMatch.partitionFormat()
	out[37] = h.PartitionMatch.signatureType()

	return out, nil
}

func unmarshalHardDrivePath(data []byte) (DevicePathElem, error) {
	var h HardDrivePath

	if len(data) != 38 {
		return nil, fmt.Errorf("invalid HardDrivePath element, expected 38 bytes, got %d", len(data))
	}

	le := binary.LittleEndian
	h.PartitionNumber = le.Uint32(data[0:4])
	h.PartitionStartBlock = le.Uint64(data[4:12])
	h.PartitionSizeBlocks = le.Uint64(data[12:20])
	partitionFormat := data[36]
	signatureType := data[37]

	var rawSig [16]byte
	copy(rawSig[:], data[20:36])

	switch {
	case partitionFormat == 1 && signatureType == 1:
		// MBR
		var mbr PartitionMBR
		copy(mbr.DiskSignature[:], rawSig[:4])
		h.PartitionMatch = mbr
	case partitionFormat == 2 && signatureType == 2:
		// GPT
		h.PartitionMatch = PartitionGPT{
			PartitionUUID: msguid.To(rawSig),
		}
	default:
		// Unknown
		h.PartitionMatch = PartitionUnknown{
			PartitionSignature: rawSig,
			PartitionFormat:    partitionFormat,
			SignatureType:      signatureType,
		}
	}

	return &h, nil
}

// FilePath contains a backslash-separated path or part of a path to a file on
// a filesystem.
type FilePath string

func (f FilePath) typ() uint8 {
	return 4
}

func (f FilePath) subType() uint8 {
	return 4
}

func (f FilePath) data() ([]byte, error) {
	if strings.IndexByte(string(f), 0x00) != -1 {
		return nil, fmt.Errorf("contains invalid null bytes")
	}

	withBackslashes := bytes.ReplaceAll([]byte(f), []byte(`/`), []byte(`\`))

	out, err := Encoding.NewEncoder().Bytes(withBackslashes)
	if err != nil {
		return nil, fmt.Errorf("failed to encode FilePath to UTF-16: %w", err)
	}

	return append(out, 0x00, 0x00), nil
}

func unmarshalFilePath(data []byte) (DevicePathElem, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("FilePath must be at least 2 bytes because of UTF-16 null terminator")
	}

	out, err := Encoding.NewDecoder().Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("error decoding FilePath UTF-16 string: %w", err)
	}

	nullIdx := bytes.IndexByte(out, 0x00)
	if nullIdx != len(out)-1 {
		return nil, fmt.Errorf("FilePath not properly null-terminated")
	}

	withoutBackslashes := strings.ReplaceAll(string(out[:len(out)-1]), `\`, `/`)

	return FilePath(withoutBackslashes), nil
}

// Map key contains type and subtype.
var pathElementUnmarshalMap = map[[2]byte]pathElemUnmarshalFunc{
	{4, 1}: unmarshalHardDrivePath,
	{4, 4}: unmarshalFilePath,
}

// UnknownPath is a generic structure for all types of path elements not
// understood by this library. The UEFI-specified set of path element
// types is vast and mostly unused, this generic type allows for parsing as
// well as pass-through of not-understood path elements.
type UnknownPath struct {
	TypeVal    uint8
	SubTypeVal uint8
	DataVal    []byte
}

func (u UnknownPath) typ() uint8 {
	return u.TypeVal
}

func (u UnknownPath) subType() uint8 {
	return u.SubTypeVal
}

func (u UnknownPath) data() ([]byte, error) {
	return u.DataVal, nil
}

// Marshal encodes the device path in binary form.
func (d DevicePath) Marshal() ([]byte, error) {
	var buf []byte //nolint:prealloc
	for _, p := range d {
		buf = append(buf, p.typ(), p.subType())

		elemBuf, err := p.data()
		if err != nil {
			return nil, fmt.Errorf("failed marshaling path element: %w", err)
		}
		// 4 is size of header which is included in length field
		if len(elemBuf)+4 > math.MaxUint16 {
			return nil, fmt.Errorf("path element payload over maximum size")
		}

		buf = binary.LittleEndian.AppendUint16(buf, uint16(len(elemBuf)+4))
		buf = append(buf, elemBuf...)
	}
	// End of device path (Type 0x7f, SubType 0xFF)
	buf = append(buf, 0x7f, 0xff, 0x04, 0x00)

	return buf, nil
}

// UnmarshalDevicePath parses a binary device path until it encounters an end
// device path structure. It returns that device path (excluding the final end
// device path marker) as well as all all data following the end marker.
//
//nolint:gocyclo
func UnmarshalDevicePath(data []byte) (DevicePath, []byte, error) {
	rest := data

	var p DevicePath

	for {
		if len(rest) < 4 {
			if len(rest) != 0 {
				return nil, nil, fmt.Errorf("dangling bytes at the end of device path: %x", rest)
			}

			break
		}

		t := rest[0]
		subT := rest[1]

		dataLen := binary.LittleEndian.Uint16(rest[2:4])
		if int(dataLen) > len(rest) {
			return nil, nil, fmt.Errorf("path element larger than rest of buffer: %d > %d", dataLen, len(rest))
		}

		if dataLen < 4 {
			return nil, nil, fmt.Errorf("path element must be at least 4 bytes (header), length indicates %d", dataLen)
		}

		elemData := rest[4:dataLen]
		rest = rest[dataLen:]

		// End of Device Path
		if t == 0x7f && subT == 0xff {
			return p, rest, nil
		}

		unmarshal, ok := pathElementUnmarshalMap[[2]byte{t, subT}]
		if !ok {
			p = append(p, &UnknownPath{
				TypeVal:    t,
				SubTypeVal: subT,
				DataVal:    elemData,
			})

			continue
		}

		elem, err := unmarshal(elemData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed decoding path element %d: %w", len(p), err)
		}

		p = append(p, elem)
	}

	if len(p) == 0 {
		return nil, nil, errors.New("empty DevicePath without End Of Path element")
	}

	return nil, nil, fmt.Errorf("got DevicePath with %d elements, but without End Of Path element", len(p))
}
