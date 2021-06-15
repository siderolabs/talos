// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package role_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/role"
)

func TestSet(t *testing.T) {
	t.Parallel()

	roles, unknownRoles := role.Parse([]string{"os:admin", "os:reader", "os:future", "os:impersonator", "", " "})
	assert.Equal(t, []string{"os:future"}, unknownRoles)
	assert.Equal(t, role.MakeSet(role.Admin, role.Reader, role.Role("os:future"), role.Impersonator), roles)

	assert.Equal(t, []string{"os:admin", "os:future", "os:impersonator", "os:reader"}, roles.Strings())
	assert.Equal(t, []string{}, role.MakeSet().Strings())

	assert.True(t, roles.Includes(role.Admin))
	assert.False(t, roles.Includes(role.Role("wrong")))

	assert.True(t, roles.IncludesAny(role.MakeSet(role.Admin)))
	assert.False(t, roles.IncludesAny(role.MakeSet(role.Role("wrong"))))

	assert.False(t, roles.IncludesAny(role.MakeSet()))
	assert.False(t, role.MakeSet().IncludesAny(roles))
	assert.False(t, role.MakeSet().IncludesAny(role.MakeSet()))
}
