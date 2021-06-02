// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz //nolint:testpackage // to test unexported method

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextPrefix(t *testing.T) {
	t.Parallel()

	for _, paths := range [][]string{
		{"/machine.MachineService/List", "/machine.MachineService", "/machine", "/", "/"},
		{"/.x", "/", "/"},
		{".", "/", "/"},
		{"./", "/", "/"},
		{"foo", "/", "/"},
		{"", "/", "/"},
	} {
		paths := paths
		t.Run(paths[0], func(t *testing.T) {
			t.Parallel()

			for i, path := range paths[:len(paths)-1] {
				expected := paths[i+1]
				actual := nextPrefix(path)
				assert.Equal(t, expected, actual, "path = %q", path)
			}
		})
	}
}
