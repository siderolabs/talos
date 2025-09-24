// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package efivarfs_test

import (
	"encoding/binary"
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
}
