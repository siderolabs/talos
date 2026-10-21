// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

package efivarfs_test

import (
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/efivarfs"
)

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return b
}

// Generated with old working marshaler and manually double-checked.
var ref = mustDecodeHex(
	"010000004a004500780061006d0070006c006500000004012a00010000000" +
		"500000000000000080000000000000014b8a76bad9dd11180b400c04fd430" +
		"c8020204041c005c0074006500730074005c0061002e00650066006900000" +
		"07fff0400",
)

func TestEncoding(t *testing.T) {
	t.Parallel()

	opt := efivarfs.LoadOption{
		Description: "Example",
		FilePath: efivarfs.DevicePath{
			&efivarfs.HardDrivePath{
				PartitionNumber:     1,
				PartitionStartBlock: 5,
				PartitionSizeBlocks: 8,
				PartitionMatch: efivarfs.PartitionGPT{
					PartitionUUID: uuid.NameSpaceX500,
				},
			},
			efivarfs.FilePath("/test/a.efi"),
		},
	}

	got, err := opt.Marshal()
	require.NoError(t, err, "failed to marshal LoadOption")

	require.Equal(t, ref, got)

	got2, err := efivarfs.UnmarshalLoadOption(got)
	require.NoError(t, err, "failed to unmarshal LoadOption")

	require.Equal(t, &opt, got2, "unmarshaled LoadOption does not match original")
}

// Descriptions exercising the edge cases of the UCS-2 null terminator search.
var descriptions = []string{
	"",
	"A",
	"Example",
	// "A" is 41 00 and U+0100 is 00 01: the bytes 00 00 in the middle
	// are not a null terminator.
	"A\u0100",
}

func FuzzDecode(f *testing.F) {
	f.Add(ref)
	f.Add(appleBootFFFF)
	f.Add(amiUEFIOSBootEntry)

	for _, description := range descriptions {
		raw, err := (&efivarfs.LoadOption{
			Description: description,
			FilePath:    efivarfs.DevicePath{efivarfs.FilePath("/test/a.efi")},
		}).Marshal()
		require.NoError(f, err)

		f.Add(raw)
	}

	f.Fuzz(func(t *testing.T, a []byte) {
		// Just try to see if it crashes
		_, _ = efivarfs.UnmarshalLoadOption(a) //nolint:errcheck
	})
}

// BootFFFF as written by the firmware of a Mac mini 2018 (Macmini8,1): an
// empty description and a device path to the macOS boot loader.
var appleBootFFFF = mustDecodeHex(
	"01000000a600000002010c00d041030a0000000001010600001b010106000000" +
		"0316100001000000000000000000000004012a0006000000102e2f0700000000" +
		"0000180000000000217485b5adb28e4fa16120dfa4e3b50e0202040450005c00" +
		"530079007300740065006d005c004c006900620072006100720079005c004300" +
		"6f0072006500530065007200760069006300650073005c0062006f006f007400" +
		"2e0065006600690000007fff0400",
)

func TestDecodeEmptyDescription(t *testing.T) {
	t.Parallel()

	got, err := efivarfs.UnmarshalLoadOption(appleBootFFFF)
	require.NoError(t, err)

	require.Equal(t, "", got.Description)
	require.False(t, got.Inactive)
	require.Len(t, got.FilePath, 6) // PciRoot, Pci, Pci, NVMe, HD, File
	require.IsType(t, &efivarfs.HardDrivePath{}, got.FilePath[4])
	require.Equal(t, efivarfs.FilePath("/System/Library/CoreServices/boot.efi"), got.FilePath[5])
	require.Empty(t, got.OptionalData)

	reencoded, err := got.Marshal()
	require.NoError(t, err)
	require.Equal(t, appleBootFFFF, reencoded)
}

// "UEFI OS" entry written by AMI firmware on a Gigabyte B85N PHOENIX-CF (BIOS F6).
// FilePathListLength is 96, but the device path ends after 94 bytes, leaving two
// zero bytes of padding after the end node.
var amiUEFIOSBootEntry = mustDecodeHex(
	"01000000600055004500460049002000" +
		"4f005300000004012a00010000000008" +
		"00000000000000a8410000000000da44" +
		"a6173c249a4c8c1f89e000db7cbc0202" +
		"040430005c004500460049005c004200" +
		"4f004f0054005c0042004f004f005400" +
		"5800360034002e004500460049000000" +
		"7fff04000000",
)

func TestDecodeTrailingPadding(t *testing.T) {
	t.Parallel()

	got, err := efivarfs.UnmarshalLoadOption(amiUEFIOSBootEntry)
	require.NoError(t, err)

	require.Equal(t, "UEFI OS", got.Description)
	require.False(t, got.Inactive)
	require.Len(t, got.FilePath, 2) // HD, File
	require.IsType(t, &efivarfs.HardDrivePath{}, got.FilePath[0])
	require.Equal(t, efivarfs.FilePath("/EFI/BOOT/BOOTX64.EFI"), got.FilePath[1])
	require.Empty(t, got.ExtraPaths)
	require.Empty(t, got.OptionalData)

	// Non-zero trailing bytes are still an error.
	garbage := append([]byte{}, amiUEFIOSBootEntry...)
	garbage[len(garbage)-1] = 0x01

	_, err = efivarfs.UnmarshalLoadOption(garbage)
	require.ErrorContains(t, err, "failed unmarshaling ExtraPath")
}

func TestDescriptionRoundTrip(t *testing.T) {
	t.Parallel()

	for _, description := range descriptions {
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			opt := efivarfs.LoadOption{
				Description: description,
				FilePath: efivarfs.DevicePath{
					efivarfs.FilePath("/test/a.efi"),
				},
				OptionalData: []byte{0x01, 0x02},
			}

			raw, err := opt.Marshal()
			require.NoError(t, err)

			got, err := efivarfs.UnmarshalLoadOption(raw)
			require.NoError(t, err)
			require.Equal(t, &opt, got)
		})
	}
}

func TestDecodeMalformedDescription(t *testing.T) {
	t.Parallel()

	opt := efivarfs.LoadOption{
		Description: "Example",
		FilePath: efivarfs.DevicePath{
			&efivarfs.HardDrivePath{
				PartitionNumber:     1,
				PartitionStartBlock: 5,
				PartitionSizeBlocks: 8,
				PartitionMatch: efivarfs.PartitionGPT{
					PartitionUUID: uuid.NameSpaceX500,
				},
			},
			efivarfs.FilePath("/test/a.efi"),
		},
	}

	raw, err := opt.Marshal()
	require.NoError(t, err)

	// Insert a byte into the description: it is now of odd length and the 00 00
	// terminator sits at an odd offset. The aligned scan must not pick up a
	// 00 00 from the HardDrivePath node instead and report a FilePath error.
	descriptionEnd := 6 + 2*len("Example")
	malformed := append(append(append([]byte{}, raw[:descriptionEnd]...), 0x41), raw[descriptionEnd:]...)

	_, err = efivarfs.UnmarshalLoadOption(malformed)
	require.ErrorContains(t, err, "end of Description")

	// Declared FilePath length larger than the whole blob.
	overrun := append([]byte{}, raw...)
	overrun[4], overrun[5] = 0xff, 0xff

	_, err = efivarfs.UnmarshalLoadOption(overrun)
	require.ErrorContains(t, err, "overruns available data")
}
