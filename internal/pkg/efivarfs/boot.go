// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

package efivarfs

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// LoadOptionCategory defines the category of a load option. This is used to
// differentiate between different types of boot options.
type LoadOptionCategory uint8

const (
	// LoadOptionCategoryBoot is the default category for boot entries.
	LoadOptionCategoryBoot LoadOptionCategory = 0x0
	// LoadOptionCategoryApp is the category for boot entries that are
	// not booted as part of the normal boot order, but are only launched via menu or hotkey.
	// This category is optional for bootloaders to support, before creating
	// new boot entries of this category firmware support needs to be
	// confirmed.
	LoadOptionCategoryApp LoadOptionCategory = 0x1
)

// LoadOption contains information on a payload to be loaded by EFI.
type LoadOption struct {
	// Human-readable description of what this load option loads.
	// This is what's being shown by the firmware when selecting a boot option.
	Description string
	// If set, firmware will skip this load option when it is in BootOrder.
	// It is unspecificed whether this prevents the user from booting the entry
	// manually.
	Inactive bool
	// If set, this load option will not be shown in any menu for load option
	// selection. This does not affect other functionality.
	Hidden bool
	// Category contains the category of the load entry. The selected category
	// affects various firmware behaviors, see the individual value
	// descriptions for more information.
	Category LoadOptionCategory
	// Path to the UEFI PE executable to execute when this load option is being
	// loaded.
	FilePath DevicePath
	// ExtraPaths contains additional device paths with vendor-specific
	// behavior. Can generally be left empty.
	ExtraPaths []DevicePath
	// OptionalData gets passed as an argument to the executed PE executable.
	// If zero-length a NULL value is passed to the executable.
	OptionalData []byte
}

// Marshal encodes a LoadOption into a binary EFI_LOAD_OPTION.
func (e *LoadOption) Marshal() ([]byte, error) {
	var (
		data  []byte
		attrs uint32
	)

	attrs |= (uint32(e.Category) & 0x1f) << 8
	if e.Hidden {
		attrs |= 0x08
	}

	if !e.Inactive {
		attrs |= 0x01
	}

	data = binary.LittleEndian.AppendUint32(data, attrs)

	filePathRaw, err := e.FilePath.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed marshaling FilePath: %w", err)
	}

	for _, ep := range e.ExtraPaths {
		epRaw, err := ep.Marshal()
		if err != nil {
			return nil, fmt.Errorf("failed marshaling ExtraPath: %w", err)
		}

		filePathRaw = append(filePathRaw, epRaw...)
	}

	if len(filePathRaw) > math.MaxUint16 {
		return nil, fmt.Errorf("failed marshaling FilePath/ExtraPath: value too big (%d)", len(filePathRaw))
	}

	data = binary.LittleEndian.AppendUint16(data, uint16(len(filePathRaw)))

	if strings.IndexByte(e.Description, 0x00) != -1 {
		return nil, fmt.Errorf("failed to encode Description: contains invalid null bytes")
	}

	encodedDesc, err := Encoding.NewEncoder().Bytes([]byte(e.Description))
	if err != nil {
		return nil, fmt.Errorf("failed to encode Description: %w", err)
	}

	data = append(data, encodedDesc...)
	data = append(data, 0x00, 0x00) // Final UTF-16/UCS-2 null code
	data = append(data, filePathRaw...)
	data = append(data, e.OptionalData...)

	return data, nil
}

// isZeroPadding reports whether data consists only of zero bytes.
func isZeroPadding(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}

	return true
}

// ucs2Terminator returns the offset of the first 00 00 null code on a 2-byte
// boundary in data, or -1 if there is none.
//
// bytes.Index is not suitable here: it also matches across characters (e.g.
// "A\u0100" encodes as 41 00 00 01), and is off by one for an empty string.
func ucs2Terminator(data []byte) int {
	for i := 0; i+1 < len(data); i += 2 {
		if data[i] == 0x00 && data[i+1] == 0x00 {
			return i
		}
	}

	return -1
}

// UnmarshalLoadOption decodes a binary EFI_LOAD_OPTION into a LoadOption.
func UnmarshalLoadOption(data []byte) (*LoadOption, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("invalid load option: minimum 6 bytes are required, got %d", len(data))
	}

	var opt LoadOption

	attrs := binary.LittleEndian.Uint32(data[:4])
	opt.Category = LoadOptionCategory((attrs >> 8) & 0x1f)
	opt.Hidden = attrs&0x08 != 0
	opt.Inactive = attrs&0x01 == 0
	lenPath := binary.LittleEndian.Uint16(data[4:6])

	if 6+int(lenPath) > len(data) {
		return nil, fmt.Errorf("declared length of FilePath (%d) overruns available data (%d)", lenPath, len(data)-6)
	}

	// The Description terminator must leave room for FilePath: bounding the
	// scan keeps it out of the device path on a malformed (odd length) description.
	descriptionEnd := ucs2Terminator(data[6 : len(data)-int(lenPath)])
	if descriptionEnd == -1 {
		return nil, fmt.Errorf("no null code point marking end of Description found before FilePath (declared length %d)", lenPath)
	}

	descriptionEnd += 6

	description, err := Encoding.NewDecoder().Bytes(data[6:descriptionEnd])
	if err != nil {
		return nil, fmt.Errorf("error decoding UTF-16 in Description: %w", err)
	}

	descriptionEnd += 2 // 2 null bytes terminating UTF-16 string
	opt.Description = string(description)

	filePathData := data[descriptionEnd : descriptionEnd+int(lenPath)]

	opt.FilePath, filePathData, err = UnmarshalDevicePath(filePathData)
	if err != nil {
		return nil, fmt.Errorf("failed unmarshaling FilePath: %w", err)
	}

	for len(filePathData) > 0 {
		// Some firmware (e.g. AMI) declares a FilePathListLength which includes
		// a few zero bytes of padding after the End of Device Path node. Those
		// are not an ExtraPath, so stop parsing there.
		if isZeroPadding(filePathData) {
			break
		}

		var extraPath DevicePath

		extraPath, filePathData, err = UnmarshalDevicePath(filePathData)
		if err != nil {
			return nil, fmt.Errorf("failed unmarshaling ExtraPath: %w", err)
		}

		opt.ExtraPaths = append(opt.ExtraPaths, extraPath)
	}

	if descriptionEnd+int(lenPath) < len(data) {
		opt.OptionalData = data[descriptionEnd+int(lenPath):]
	}

	return &opt, nil
}

// BootOrder represents the contents of the BootOrder EFI variable.
type BootOrder []uint16

// Marshal generates the binary representation of a BootOrder.
func (t *BootOrder) Marshal() []byte {
	var out []byte

	for _, v := range *t {
		out = binary.LittleEndian.AppendUint16(out, v)
	}

	return out
}

// UnmarshalBootOrder loads a BootOrder from its binary representation.
func UnmarshalBootOrder(d []byte) (BootOrder, error) {
	if len(d)%2 != 0 {
		return nil, fmt.Errorf("invalid length: %v bytes", len(d))
	}

	l := len(d) / 2

	out := make(BootOrder, l)
	for i := range l {
		out[i] = binary.LittleEndian.Uint16(d[i*2:])
	}

	return out, nil
}
