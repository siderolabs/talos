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

func TestParseHostIDList(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		list  string
		maxID int

		expected      string
		expectedError string
	}{
		{name: "empty", list: "", maxID: hypervisorhelpers.MaxHostCPUID, expected: ""},
		{name: "single", list: "8", maxID: hypervisorhelpers.MaxHostCPUID, expected: "8"},
		{name: "canonical", list: "10,9,1-3,2-4,+1", maxID: hypervisorhelpers.MaxHostCPUID, expected: "1-4,9-10"},
		{name: "top CPU", list: "999", maxID: hypervisorhelpers.MaxHostCPUID, expected: "999"},
		{name: "top NUMA node", list: "16383", maxID: hypervisorhelpers.MaxHostNUMANodeID, expected: "16383"},
		{
			name: "past top CPU", list: "1000", maxID: hypervisorhelpers.MaxHostCPUID,
			expectedError: "1000 is out of range, IDs must be between 0 and 999",
		},
		{
			name: "past top NUMA node", list: "16384", maxID: hypervisorhelpers.MaxHostNUMANodeID,
			expectedError: "16384 is out of range, IDs must be between 0 and 16383",
		},
		{
			name: "range end past top", list: "0-1000", maxID: hypervisorhelpers.MaxHostCPUID,
			expectedError: "1000 is out of range, IDs must be between 0 and 999",
		},
		{
			// The bound must be checked before the range is expanded, or this would allocate a
			// billion elements.
			name: "huge range", list: "0-1000000000", maxID: hypervisorhelpers.MaxHostCPUID,
			expectedError: "1000000000 is out of range, IDs must be between 0 and 999",
		},
		{
			name: "huge range after a valid one", list: "1,0-1000000000", maxID: hypervisorhelpers.MaxHostCPUID,
			expectedError: "1000000000 is out of range, IDs must be between 0 and 999",
		},
		{name: "negative", list: "-1", maxID: hypervisorhelpers.MaxHostCPUID, expectedError: "invalid syntax"},
		{name: "reversed", list: "3-1", maxID: hypervisorhelpers.MaxHostCPUID, expectedError: "invalid range"},
		{name: "letters", list: "a", maxID: hypervisorhelpers.MaxHostCPUID, expectedError: "invalid syntax"},
		{name: "whitespace", list: "1, 2", maxID: hypervisorhelpers.MaxHostCPUID, expectedError: "invalid syntax"},
		{name: "exclusion", list: "0-3,^1", maxID: hypervisorhelpers.MaxHostCPUID, expectedError: "invalid syntax"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			set, err := hypervisorhelpers.ParseHostIDList(test.list, test.maxID)

			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)
				assert.True(t, set.IsEmpty())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, set.String())
		})
	}
}
