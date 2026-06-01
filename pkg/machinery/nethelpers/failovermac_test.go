// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

func TestFailOverMACUnmarshalText(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		input    string
		expected nethelpers.FailOverMAC
	}{
		{
			input:    "none",
			expected: nethelpers.FailOverMACNone,
		},
		{
			input:    "active",
			expected: nethelpers.FailOverMACActive,
		},
		{
			input:    "follow",
			expected: nethelpers.FailOverMACFollow,
		},
		{
			input:    "",
			expected: nethelpers.FailOverMACNone,
		},
		{
			input:    "0",
			expected: nethelpers.FailOverMACNone,
		},
		{
			input:    "1",
			expected: nethelpers.FailOverMACActive,
		},
		{
			input:    "2",
			expected: nethelpers.FailOverMACFollow,
		},
	} {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()

			var f nethelpers.FailOverMAC

			err := f.UnmarshalText([]byte(test.input))
			require.NoError(t, err)

			assert.Equal(t, test.expected, f)
		})
	}
}
