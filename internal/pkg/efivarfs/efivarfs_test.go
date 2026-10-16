// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package efivarfs_test

import (
	"encoding/binary"
	"encoding/hex"
	"io/fs"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/efivarfs"
)

func TestBootOrder(t *testing.T) {
	t.Parallel()

	var bootOrderEntries []byte

	for _, entry := range []int{1, 0, 2, 3} {
		bootOrderEntries = binary.LittleEndian.AppendUint16(bootOrderEntries, uint16(entry))
	}

	efiRW := efivarfs.Mock{
		Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
			efivarfs.ScopeGlobal: {
				"BootOrder": {
					Attrs: 0,
					Data:  bootOrderEntries,
				},
			},
		},
	}

	vars, err := efiRW.List(efivarfs.ScopeGlobal)
	require.NoError(t, err)

	require.Contains(t, vars, "BootOrder", "variable BootOrder not found")

	bootOrder, err := efivarfs.GetBootOrder(&efiRW)
	require.NoError(t, err)

	require.Equal(t, efivarfs.BootOrder([]uint16{1, 0, 2, 3}), bootOrder, "BootOrder does not match expected value")

	require.NoError(t, efivarfs.SetBootOrder(&efiRW, efivarfs.BootOrder([]uint16{1, 0, 3})))

	bootOrder, err = efivarfs.GetBootOrder(&efiRW)
	require.NoError(t, err)

	require.Equal(t, efivarfs.BootOrder([]uint16{1, 0, 3}), bootOrder, "BootOrder does not match expected value after SetBootOrder")
}

func TestBootEntries(t *testing.T) {
	t.Parallel()

	efiRW := efivarfs.Mock{}

	// no entries yet
	entries, err := efivarfs.ListBootEntries(&efiRW)
	require.NoError(t, err)
	require.Empty(t, len(entries), "expected no boot entries in empty mock")

	// create first entry
	idx, err := efivarfs.AddBootEntry(&efiRW, &efivarfs.LoadOption{
		Description: "First Entry",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/first.efi"),
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0, idx, "first boot entry index should be 0")

	// verify first entry
	entry, err := efivarfs.GetBootEntry(&efiRW, idx)
	require.NoError(t, err)
	require.Equal(t, "First Entry", entry.Description, "first boot entry description does not match")
	require.Equal(t, efivarfs.DevicePath{efivarfs.FilePath("/first.efi")}, entry.FilePath, "first boot entry file path does not match")

	// create second entry
	require.NoError(t, efivarfs.SetBootEntry(&efiRW, 1, &efivarfs.LoadOption{
		Description: "Second Entry",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/second.efi"),
		},
	}), "failed to set second boot entry")

	// verify second entry
	entry, err = efivarfs.GetBootEntry(&efiRW, 1)
	require.NoError(t, err)
	require.Equal(t, "Second Entry", entry.Description, "second boot entry description does not match")
	require.Equal(t, efivarfs.DevicePath{efivarfs.FilePath("/second.efi")}, entry.FilePath, "second boot entry file path does not match")

	// list all entries
	entries, err = efivarfs.ListBootEntries(&efiRW)
	require.NoError(t, err)
	require.Len(t, entries, 2, "expected exactly two boot entries after adding two")

	// try overwrite first entry
	require.NoError(t, efivarfs.SetBootEntry(&efiRW, idx, &efivarfs.LoadOption{
		Description: "First Entry Overwritten",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/first_overwritten.efi"),
		},
	}), "failed to overwrite first boot entry")

	// verify first entry after overwrite
	entry, err = efivarfs.GetBootEntry(&efiRW, idx)
	require.NoError(t, err)
	require.Equal(t, "First Entry Overwritten", entry.Description, "first boot entry description does not match after overwrite")
	require.Equal(t, efivarfs.DevicePath{efivarfs.FilePath("/first_overwritten.efi")}, entry.FilePath, "first boot entry file path does not match after overwrite")

	// verify delete non-existing entry
	require.ErrorIs(t, efivarfs.DeleteBootEntry(&efiRW, 42), fs.ErrNotExist, "expected ErrNoSuchEntry when deleting non-existing entry")

	// delete second entry
	require.NoError(t, efivarfs.DeleteBootEntry(&efiRW, 1), "failed to delete second boot entry")

	// verify second entry is gone
	_, err = efivarfs.GetBootEntry(&efiRW, 1)
	require.ErrorIs(t, err, fs.ErrNotExist, "expected ErrNoSuchEntry when getting deleted entry")

	// list entries
	entries, err = efivarfs.ListBootEntries(&efiRW)
	require.NoError(t, err)
	require.Len(t, entries, 1, "expected exactly one boot entry after deleting one of two")

	// set entry with a high index
	require.NoError(t, efivarfs.SetBootEntry(&efiRW, 42, &efivarfs.LoadOption{
		Description: "High Index Entry",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/high_index.efi"),
		},
	}), "failed to set high index boot entry")

	// make sure adding a new entry uses the lowest available index (which is 1 now)
	newIdx, err := efivarfs.AddBootEntry(&efiRW, &efivarfs.LoadOption{
		Description: "New Entry",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/new.efi"),
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, newIdx, "expected new boot entry index to be 1, the lowest available index")
}

func TestUniqueBootOrder(t *testing.T) {
	t.Parallel()

	require.Equal(t, efivarfs.BootOrder{}, efivarfs.UniqueBootOrder(efivarfs.BootOrder{}), "empty BootOrder should remain empty")

	require.Equal(t, efivarfs.BootOrder{1, 2, 3}, efivarfs.UniqueBootOrder(efivarfs.BootOrder{1, 2, 3}), "BootOrder with unique entries should remain unchanged")

	require.Equal(t, efivarfs.BootOrder{1, 2, 3}, efivarfs.UniqueBootOrder(efivarfs.BootOrder{1, 2, 3, 2, 1, 3}), "BootOrder with duplicates should have duplicates removed preserving order of first appearance") //nolint:lll

	require.Equal(t, efivarfs.BootOrder{0, 1, 3, 2}, efivarfs.UniqueBootOrder(efivarfs.BootOrder{0, 1, 0, 1, 0, 1, 1, 3, 2, 2, 3, 3, 1}), "BootOrder with all entries duplicated should have duplicates removed preserving order of first appearance") //nolint:lll
}

// "UEFI OS" entry written by AMI firmware on a Gigabyte B85N PHOENIX-CF (BIOS F6).
// FilePathListLength is 96, but the device path ends after 94 bytes, leaving two
// zero bytes after the end node.
const amiUEFIOSBootEntry = "01000000600055004500460049002000" +
	"4f005300000004012a00010000000008" +
	"00000000000000a8410000000000da44" +
	"a6173c249a4c8c1f89e000db7cbc0202" +
	"040430005c004500460049005c004200" +
	"4f004f0054005c0042004f004f005400" +
	"5800360034002e004500460049000000" +
	"7fff04000000"

func TestListBootEntriesUndecodable(t *testing.T) {
	t.Parallel()

	undecodable, err := hex.DecodeString(amiUEFIOSBootEntry)
	require.NoError(t, err)

	_, err = efivarfs.UnmarshalLoadOption(undecodable)
	require.ErrorContains(t, err, "dangling bytes at the end of device path")

	valid, err := (&efivarfs.LoadOption{
		Description: "Valid",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/valid.efi"),
		},
	}).Marshal()
	require.NoError(t, err)

	efiRW := efivarfs.Mock{
		Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
			efivarfs.ScopeGlobal: {
				"Boot0000": {Data: undecodable},
				"Boot0001": {Data: valid},
			},
		},
	}

	entries, err := efivarfs.ListBootEntries(&efiRW)
	require.NoError(t, err)

	require.Len(t, entries, 2)
	require.Contains(t, entries, 0)
	require.Nil(t, entries[0])
	require.NotNil(t, entries[1])
	require.Equal(t, "Valid", entries[1].Description)

	// The index of the undecodable entry must stay occupied.
	idx, err := efivarfs.AddBootEntry(&efiRW, &efivarfs.LoadOption{
		Description: "New",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/new.efi"),
		},
	})
	require.NoError(t, err)
	require.Equal(t, 2, idx)
	require.Equal(t, undecodable, efiRW.Variables[efivarfs.ScopeGlobal]["Boot0000"].Data)
}

func TestListBootEntriesUndecodableDuplicateCase(t *testing.T) {
	t.Parallel()

	undecodable, err := hex.DecodeString(amiUEFIOSBootEntry)
	require.NoError(t, err)

	valid, err := (&efivarfs.LoadOption{
		Description: "Valid",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/valid.efi"),
		},
	}).Marshal()
	require.NoError(t, err)

	// Both spellings of the same index: the decodable one must win,
	// regardless of the order the variables are listed in.
	efiRW := efivarfs.Mock{
		Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
			efivarfs.ScopeGlobal: {
				"Boot000A": {Data: valid},
				"Boot000a": {Data: undecodable},
			},
		},
	}

	entries, err := efivarfs.ListBootEntries(&efiRW)
	require.NoError(t, err)

	require.Len(t, entries, 1)
	require.NotNil(t, entries[10])
	require.Equal(t, "Valid", entries[10].Description)
}
