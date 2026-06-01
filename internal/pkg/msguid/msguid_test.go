// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

package msguid_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/msguid"
)

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		uuid     string
		expected [16]byte
	}{
		{
			"WikipediaExample1",
			"00112233-4455-6677-8899-AABBCCDDEEFF",
			[16]byte{
				0x33, 0x22, 0x11, 0x00, 0x55, 0x44, 0x77, 0x66,
				0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			origUUID := uuid.MustParse(c.uuid)
			got := msguid.From(origUUID)

			require.Equal(t, c.expected, got, "From(%q) returned unexpected result", origUUID)

			back := msguid.To(got)

			require.Equal(t, origUUID, back, "From(To(%q)) did not return original value", origUUID)
		})
	}
}
