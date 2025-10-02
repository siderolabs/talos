// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sdboot_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/internal/pkg/efivarfs"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

type mockLogger struct {
	strings.Builder
}

func (m *mockLogger) Printf(format string, v ...any) {
	m.WriteString(fmt.Sprintf(format, v...) + "\n")
}

func TestSetBootEntry(t *testing.T) {
	t.Parallel()

	loadOption := &efivarfs.LoadOption{
		Description: "Default Boot Entry",
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/default.efi"),
		},
	}

	defaultBootEntry, err := loadOption.Marshal()
	require.NoError(t, err)

	talosLoadOption := &efivarfs.LoadOption{
		Description: sdboot.TalosBootEntryDescription,
		FilePath: efivarfs.DevicePath{
			efivarfs.FilePath("/EFI/TALOS/UKI.efi"),
		},
	}

	talosBootEntry, err := talosLoadOption.Marshal()
	require.NoError(t, err)

	blkidInfo := &blkid.Info{
		ProbeResult: blkid.ProbeResult{
			Name: "loop0",
		},
		SectorSize: 512,
		Parts: []blkid.NestedProbeResult{
			{
				NestedResult: blkid.NestedResult{
					PartitionUUID:   pointer.To(uuid.MustParse("3c8f4e2e-1dd2-4a5b-9f6d-8f3c9e6d7c3b")),
					PartitionLabel:  pointer.To(constants.EFIPartitionLabel),
					PartitionOffset: 2048,
					PartitionSize:   409600,
					PartitionIndex:  1,
					PartitionType:   pointer.To(uuid.MustParse("c12a7328-f81f-11d2-ba4b-00a0c93ec93b")),
				},
			},
		},
	}

	for _, testData := range []struct {
		name         string
		efivarfsMock *efivarfs.Mock

		expectedMessageContains string
		expectedBootOrder       efivarfs.BootOrder
		expectedEntries         map[int]string
	}{
		{
			name:         "empty efivarfs", // both BootOrder and BootEntries are initially empty
			efivarfsMock: &efivarfs.Mock{},

			expectedMessageContains: "setting Talos Linux UKI boot entry at index 0 as first in BootOrder: [0]",
			expectedBootOrder:       efivarfs.BootOrder{0},
			expectedEntries: map[int]string{
				0: sdboot.TalosBootEntryDescription,
			},
		},
		{
			name: "existing BootEntry but empty BootOrder", // BootOrder is empty but there is already a BootEntry
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"Boot0000": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
					},
				},
			},

			expectedMessageContains: "setting Talos Linux UKI boot entry at index 1 as first in BootOrder: [1]",
			expectedBootOrder:       efivarfs.BootOrder{1},
			expectedEntries: map[int]string{
				0: "Default Boot Entry",
				1: sdboot.TalosBootEntryDescription,
			},
		},
		{
			name: "existing BootOrder but empty BootEntries", // BootOrder has an entry but there are no BootEntries
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x00, 0x00},
						},
					},
				},
			},

			expectedMessageContains: "Talos Linux UKI boot entry at index 0 is already first in BootOrder: [0]",
			expectedBootOrder:       efivarfs.BootOrder{0},
			expectedEntries: map[int]string{
				0: sdboot.TalosBootEntryDescription,
			},
		},
		{
			name: "existing BootOrder and BootEntries matching", // both BootOrder and BootEntries have an entry and they match
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x00, 0x00}, // BootOrder: [0]
						},
						"Boot0000": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
					},
				},
			},

			expectedMessageContains: "setting Talos Linux UKI boot entry at index 1 as first in BootOrder: [1 0]",
			expectedBootOrder:       efivarfs.BootOrder{1, 0},
			expectedEntries: map[int]string{
				0: "Default Boot Entry",
				1: sdboot.TalosBootEntryDescription,
			},
		},
		{
			name: "existing BootOrder and BootEntries not matching", // both BootOrder and BootEntries have an entry but they don't match
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x01, 0x00}, // BootOrder: [1]
						},
						"Boot0000": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
					},
				},
			},

			expectedMessageContains: "Talos Linux UKI boot entry at index 1 is already first in BootOrder: [1]",
			expectedBootOrder:       efivarfs.BootOrder{1},
			expectedEntries: map[int]string{
				0: "Default Boot Entry",
				1: sdboot.TalosBootEntryDescription,
			},
		},
		{
			name: "existing BootOrder and BootEntries not matching multiple", // both BootOrder and BootEntries have an entry but they don't match
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x01, 0x00, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00}, // BootOrder: [1, 0, 3, 2]
						},
						"Boot0000": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
						"Boot0002": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
					},
				},
			},

			expectedMessageContains: "Talos Linux UKI boot entry at index 1 is already first in BootOrder: [1 0 3 2]",
			expectedBootOrder:       efivarfs.BootOrder{1, 0, 3, 2},
			expectedEntries: map[int]string{
				0: "Default Boot Entry",
				1: sdboot.TalosBootEntryDescription,
				2: "Default Boot Entry",
			},
		},
		{
			name: "existing BootOrder and BootEntries not matching multiple-1", // both BootOrder and BootEntries have an entry but they don't match
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x05, 0x00, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00}, // BootOrder: [5, 0, 3, 2]
						},
						"Boot0000": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
						"Boot0003": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
					},
				},
			},

			expectedMessageContains: "setting Talos Linux UKI boot entry at index 1 as first in BootOrder: [1 5 0 3 2]",
			expectedBootOrder:       efivarfs.BootOrder{1, 5, 0, 3, 2},
			expectedEntries: map[int]string{
				0: "Default Boot Entry",
				1: sdboot.TalosBootEntryDescription,
				3: "Default Boot Entry",
			},
		},
		{
			name: "duplicate entries in BootOrder but not BootEntries", // BootOrder has duplicate entries and no matching BootEntries
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00, 0x03, 0x00}, // BootOrder: [1, 0, 0, 3, 2, 3]
						},
					},
				},
			},

			expectedMessageContains: "setting Talos Linux UKI boot entry at index 0 as first in BootOrder: [0 1 3 2]",
			expectedBootOrder:       efivarfs.BootOrder{0, 1, 3, 2},
			expectedEntries: map[int]string{
				0: sdboot.TalosBootEntryDescription,
			},
		},
		{
			name: "duplicate Talos entries in BootEntries", // BootOrder has unique entries but there are multiple Talos BootEntries
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x01, 0x00, 0x02, 0x00}, // BootOrder: [1, 2]
						},
						"Boot0000": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
						"Boot0001": {
							Attrs: 0,
							Data:  talosBootEntry,
						},
						"Boot0002": {
							Attrs: 0,
							Data:  talosBootEntry,
						},
					},
				},
			},

			expectedMessageContains: "Removing existing Talos Linux UKI boot entry at index 2",
			expectedBootOrder:       efivarfs.BootOrder{1},
			expectedEntries: map[int]string{
				0: "Default Boot Entry",
				1: sdboot.TalosBootEntryDescription,
			},
		},
		{
			name: "duplicate Talos entries in BootEntries and duplicate BootOrder", // BootOrder has duplicate entries and there are multiple Talos BootEntries
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x01, 0x00, 0x02, 0x00, 0x02, 0x00, 0x00, 0x00}, // BootOrder: [1, 2, 2, 0]
						},
						"Boot0000": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
						"Boot0001": {
							Attrs: 0,
							Data:  talosBootEntry,
						},
						"Boot0002": {
							Attrs: 0,
							Data:  talosBootEntry,
						},
					},
				},
			},

			expectedMessageContains: "Removing existing Talos Linux UKI boot entry at index 2",
			expectedBootOrder:       efivarfs.BootOrder{1, 0},
			expectedEntries: map[int]string{
				0: "Default Boot Entry",
				1: sdboot.TalosBootEntryDescription,
			},
		},
		{
			name: "duplicate entries in BootOrder and BootEntries", // BootOrder has duplicate entries and has multiple BootEntries
			efivarfsMock: &efivarfs.Mock{
				Variables: map[uuid.UUID]map[string]efivarfs.MockVariable{
					efivarfs.ScopeGlobal: {
						"BootOrder": {
							Attrs: 0,
							Data:  []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00, 0x03, 0x00}, // BootOrder: [1, 0, 0, 3, 2, 3]
						},
						"Boot0000": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
						"Boot0001": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
						"Boot0003": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
						"Boot002a": {
							Attrs: 0,
							Data:  defaultBootEntry,
						},
					},
				},
			},

			expectedMessageContains: "setting Talos Linux UKI boot entry at index 2 as first in BootOrder: [2 1 0 3]",
			expectedBootOrder:       efivarfs.BootOrder{2, 1, 0, 3},
			expectedEntries: map[int]string{
				0:  "Default Boot Entry",
				1:  "Default Boot Entry",
				2:  sdboot.TalosBootEntryDescription,
				3:  "Default Boot Entry",
				42: "Default Boot Entry",
			},
		},
	} {
		t.Run(testData.name, func(t *testing.T) {
			t.Parallel()

			if testData.efivarfsMock == nil {
				t.Fatal("efivarfsMock must be set")
			}

			logger := &mockLogger{}

			require.NoError(t, sdboot.CreateBootEntry(testData.efivarfsMock, blkidInfo, logger.Printf, "test-entry"))

			bootOrder, err := efivarfs.GetBootOrder(testData.efivarfsMock)
			require.NoError(t, err)

			require.Contains(t, logger.String(), testData.expectedMessageContains, "expected log message not found")

			require.Equal(t, testData.expectedBootOrder, bootOrder, "BootOrder does not match expected value")

			bootEntries, err := efivarfs.ListBootEntries(testData.efivarfsMock)
			require.NoError(t, err)

			require.Len(t, bootEntries, len(testData.expectedEntries), "number of boot entries does not match expected value")

			for idx, desc := range testData.expectedEntries {
				entry, err := efivarfs.GetBootEntry(testData.efivarfsMock, idx)
				require.NoError(t, err)

				require.Equal(t, desc, entry.Description, "boot entry description does not match expected value")
			}
		})
	}
}
