// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

// Package msguid provides functions to convert UUIDs/GUIDs to and from
// Microsoft's idiosyncratic "mixed-endian" format.
// See https://uefi.org/specs/UEFI/2.10/Apx_A_GUID_and_Time_Formats.html#text-representation-relationships-apxa-guid-and-time-formats
// for an explanation of the format.
package msguid

import "github.com/google/uuid"

var mixedEndianTranspose = []int{3, 2, 1, 0, 5, 4, 7, 6, 8, 9, 10, 11, 12, 13, 14, 15}

// From converts from a standard UUID into its mixed-endian encoding.
func From(u uuid.UUID) (o [16]byte) {
	for dest, from := range mixedEndianTranspose {
		o[dest] = u[from]
	}

	return o
}

// To converts a mixed-endian-encoded UUID to its standard format.
func To(i [16]byte) (o uuid.UUID) {
	for from, dest := range mixedEndianTranspose {
		o[dest] = i[from]
	}

	return o
}
