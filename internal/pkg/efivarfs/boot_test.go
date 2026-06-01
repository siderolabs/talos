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

// Generated with old working marshaler and manually double-checked
//
// nolint:errcheck
var ref, _ = hex.DecodeString(
	"010000004a004500780061006d0070006c006500000004012a00010000000" +
		"500000000000000080000000000000014b8a76bad9dd11180b400c04fd430" +
		"c8020204041c005c0074006500730074005c0061002e00650066006900000" +
		"07fff0400",
)

func TestEncoding(t *testing.T) {
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

func FuzzDecode(f *testing.F) {
	f.Add(ref)
	f.Fuzz(func(t *testing.T, a []byte) {
		// Just try to see if it crashes
		_, _ = efivarfs.UnmarshalLoadOption(a) //nolint:errcheck
	})
}
