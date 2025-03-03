// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/grpc/proxy/backend"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

func TestLocalGetConnection(t *testing.T) {
	t.Parallel()

	l := backend.NewLocal("test", "/tmp/test.sock")

	md1 := metadata.New(nil)
	md1.Set("key", "value1", "value2")
	ctx1 := metadata.NewIncomingContext(authz.ContextWithRoles(t.Context(), role.MakeSet(role.Admin)), md1)

	outCtx1, conn1, err1 := l.GetConnection(ctx1, "")
	assert.NoError(t, err1)
	assert.NotNil(t, conn1)
	assert.Equal(t, role.MakeSet(role.Admin), authz.GetRoles(outCtx1))

	mdOut1, ok1 := metadata.FromOutgoingContext(outCtx1)
	assert.True(t, ok1)
	assert.Equal(t, []string{"value1", "value2"}, mdOut1.Get("key"))
	assert.Equal(t, []string{"os:admin"}, mdOut1.Get("talos-role"))

	t.Run("Same context", func(t *testing.T) {
		t.Parallel()

		ctx2 := ctx1
		outCtx2, conn2, err2 := l.GetConnection(ctx2, "")
		assert.NoError(t, err2)
		assert.Equal(t, conn1, conn2) // connection is cached
		assert.Equal(t, role.MakeSet(role.Admin), authz.GetRoles(outCtx2))

		mdOut2, ok2 := metadata.FromOutgoingContext(outCtx2)
		assert.True(t, ok2)
		assert.Equal(t, []string{"value1", "value2"}, mdOut2.Get("key"))
		assert.Equal(t, []string{"os:admin"}, mdOut2.Get("talos-role"))
	})

	t.Run("Other context", func(t *testing.T) {
		t.Parallel()

		md3 := metadata.New(nil)
		md3.Set("key", "value3", "value4")
		ctx3 := metadata.NewIncomingContext(authz.ContextWithRoles(t.Context(), role.MakeSet(role.Reader)), md3)

		outCtx3, conn3, err3 := l.GetConnection(ctx3, "")
		assert.NoError(t, err3)
		assert.Equal(t, conn1, conn3) // connection is cached
		assert.Equal(t, role.MakeSet(role.Reader), authz.GetRoles(outCtx3))

		mdOut3, ok3 := metadata.FromOutgoingContext(outCtx3)
		assert.True(t, ok3)
		assert.Equal(t, []string{"value3", "value4"}, mdOut3.Get("key"))
		assert.Equal(t, []string{"os:reader"}, mdOut3.Get("talos-role"))
	})
}
