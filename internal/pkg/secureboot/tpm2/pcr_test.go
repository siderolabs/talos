// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	tpm2internal "github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
)

func TestGetSelection(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		pcrs     []int
		expected []byte
	}{
		{
			name:     "empty",
			expected: []byte{0, 0, 0},
		},
		{
			name:     "1, 3, 5",
			pcrs:     []int{1, 3, 5},
			expected: []byte{42, 0, 0},
		},
		{
			name:     "21, 22, 23",
			pcrs:     []int{21, 22, 23},
			expected: []byte{0, 0, 0xe0},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := tpm2internal.CreateSelector(tt.pcrs)
			require.NoError(t, err)

			require.Equal(t, tt.expected, actual)
		})
	}
}
