// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package role_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/role"
)

func TestRole(t *testing.T) {
	t.Parallel()

	set, err := role.Parse([]string{"os:admin", "os:reader", "os:future", "os:impersonator", "", " "})
	assert.EqualError(t, err, "1 error occurred:\n\t* unexpected role \"os:future\"\n\n")
	assert.Equal(t, role.MakeSet(role.Admin, role.Reader, role.Role("os:future"), role.Impersonator), set)

	assert.Equal(t, []string{"os:admin", "os:future", "os:impersonator", "os:reader"}, set.Strings())
	assert.Equal(t, []string{}, role.Set.Strings(nil))

	_, ok := set[role.Admin]
	assert.True(t, ok)
	assert.True(t, set.IncludesAny(role.MakeSet(role.Admin)))
	assert.False(t, set.IncludesAny(nil))
}
