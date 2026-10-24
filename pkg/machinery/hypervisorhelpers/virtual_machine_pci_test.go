// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

func TestParsePCIAddress(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		address string

		expected      hypervisorhelpers.PCIAddress
		canonical     string
		expectedError string
	}{
		{
			name:      "canonical",
			address:   "0000:41:00.0",
			expected:  hypervisorhelpers.PCIAddress{Bus: 0x41},
			canonical: "0000:41:00.0",
		},
		{
			name:      "uppercase",
			address:   "0001:AF:1F.7",
			expected:  hypervisorhelpers.PCIAddress{Domain: 1, Bus: 0xaf, Slot: 0x1f, Function: 7},
			canonical: "0001:af:1f.7",
		},
		{
			name:          "missing domain",
			address:       "41:00.0",
			expectedError: `PCI address "41:00.0" must be in the form dddd:bb:ss.f, e.g. 0000:41:00.0`,
		},
		{
			name:          "function out of range",
			address:       "0000:41:00.8",
			expectedError: `PCI address "0000:41:00.8" must be in the form dddd:bb:ss.f, e.g. 0000:41:00.0`,
		},
		{
			name:          "slot out of range",
			address:       "0000:41:20.0",
			expectedError: `PCI address "0000:41:20.0": slot must be between 00 and 1f`,
		},
		{
			name:          "trailing garbage",
			address:       "0000:41:00.0/",
			expectedError: `PCI address "0000:41:00.0/" must be in the form dddd:bb:ss.f, e.g. 0000:41:00.0`,
		},
		{
			name:          "empty",
			address:       "",
			expectedError: `PCI address "" must be in the form dddd:bb:ss.f, e.g. 0000:41:00.0`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			address, err := hypervisorhelpers.ParsePCIAddress(test.address)
			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, address)
			assert.Equal(t, test.canonical, address.String())
		})
	}
}
