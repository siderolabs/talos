// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dmmultipath_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/dmmultipath"
)

func TestParsePathStates(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		status   string
		expected map[string]bool
	}{
		{
			name:   "round-robin, one path failed",
			status: "2 0 0 0 1 1 A 0 2 0 8:0 A 0 8:16 F 1 ",
			expected: map[string]bool{
				"8:0":  true,
				"8:16": false,
			},
		},
		{
			name: "service-time with selector status, two path groups",
			// path groups carry their own state ("A", "E") right before the selector status, and
			// service-time reports two extra fields per path
			status: "2 0 0 0 2 1 A 0 1 2 8:0 A 0 0 1 E 0 1 2 8:16 F 3 0 1 ",
			expected: map[string]bool{
				"8:0":  true,
				"8:16": false,
			},
		},
		{
			name:     "no paths",
			status:   "2 0 0 0 0 0 ",
			expected: map[string]bool{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, dmmultipath.ParsePathStates(test.status))
		})
	}
}
