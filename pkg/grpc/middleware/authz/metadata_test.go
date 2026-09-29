// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authz_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

func TestSetMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		roles    role.Set
		expected []string
	}{
		{
			name:     "single role",
			roles:    role.MakeSet(role.Reader),
			expected: []string{"os:reader"},
		},
		{
			name:     "multiple roles are sorted",
			roles:    role.MakeSet(role.Reader, role.Admin),
			expected: []string{"os:admin", "os:reader"},
		},
		{
			name:     "empty role set removes the key",
			roles:    role.Zero,
			expected: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			md := metadata.New(map[string]string{"talos-role": "os:admin"})

			authz.SetMetadata(md, test.roles)

			assert.Equal(t, test.expected, md.Get("talos-role"))
		})
	}

	t.Run("empty role set on empty metadata", func(t *testing.T) {
		t.Parallel()

		md := metadata.MD{}

		authz.SetMetadata(md, role.Zero)

		assert.Empty(t, md.Get("talos-role"))
	})
}
