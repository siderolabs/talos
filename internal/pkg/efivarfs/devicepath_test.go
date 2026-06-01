// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

package efivarfs_test

import (
	"bytes"
	"testing"

	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/pkg/efivarfs"
)

// nolint:gocyclo
func TestMarshalExamples(t *testing.T) {
	cases := []struct {
		name        string
		path        efivarfs.DevicePath
		expected    []byte
		expectError bool
	}{
		{
			name: "TestNone",
			path: efivarfs.DevicePath{},
			expected: []byte{
				0x7f, 0xff, // End of HW device path
				0x04, 0x00, // Length: 4 bytes
			},
		},
		{
			// From UEFI Device Path Examples, extracted single entry
			name: "TestHD",
			path: efivarfs.DevicePath{
				&efivarfs.HardDrivePath{
					PartitionNumber:     1,
					PartitionStartBlock: 0x22,
					PartitionSizeBlocks: 0x2710000,
					PartitionMatch: efivarfs.PartitionGPT{
						PartitionUUID: uuid.MustParse("15E39A00-1DD2-1000-8D7F-00A0C92408FC"),
					},
				},
			},
			expected: []byte{
				0x04, 0x01, // Hard Disk type
				0x2a, 0x00, // Length
				0x01, 0x00, 0x00, 0x00, // Partition Number
				0x22, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Part Start
				0x00, 0x00, 0x71, 0x02, 0x00, 0x00, 0x00, 0x00, // Part Size
				0x00, 0x9a, 0xe3, 0x15, 0xd2, 0x1d, 0x00, 0x10,
				0x8d, 0x7f, 0x00, 0xa0, 0xc9, 0x24, 0x08, 0xfc, // Signature
				0x02,       // Part Format GPT
				0x02,       // Signature GPT
				0x7f, 0xff, // End of HW device path
				0x04, 0x00, // Length: 4 bytes
			},
		},
		{
			name: "TestFilePath",
			path: efivarfs.DevicePath{
				efivarfs.FilePath("asdf"),
			},
			expected: []byte{
				0x04, 0x04, // File Path type
				0x0e, 0x00, // Length
				'a', 0x00, 's', 0x00, 'd', 0x00, 'f', 0x00,
				0x00, 0x00,
				0x7f, 0xff, // End of HW device path
				0x04, 0x00, // Length: 4 bytes
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.path.Marshal()
			if err != nil && !c.expectError {
				t.Fatalf("unexpected error: %v", err)
			}

			if err == nil && c.expectError {
				t.Fatalf("expected error, got %x", got)
			}

			if err != nil && c.expectError {
				// Do not compare result in case error is expected
				return
			}

			if !bytes.Equal(got, c.expected) {
				t.Fatalf("expected %x, got %x", c.expected, got)
			}

			_, rest, err := efivarfs.UnmarshalDevicePath(got)
			if err != nil {
				t.Errorf("failed to unmarshal value again: %v", err)
			}

			if len(rest) != 0 {
				t.Errorf("rest is non-zero after single valid device path: %x", rest)
			}
		})
	}
}
