// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/pkg/grpc/proxy/backend"
)

func TestLocalInterfaces(t *testing.T) {
	assert.Implements(t, (*proxy.Backend)(nil), new(backend.Local))
}

func TestLocalGetConnection(t *testing.T) {
	l := backend.NewLocal("test", "/tmp/test.sock")

	md := metadata.New(nil)
	md.Set("key", "value1", "value2")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	outCtx1, conn1, err1 := l.GetConnection(ctx)
	assert.NoError(t, err1)
	assert.NotNil(t, conn1)

	mdOut1, ok1 := metadata.FromOutgoingContext(outCtx1)
	assert.True(t, ok1)
	assert.Equal(t, []string{"value1", "value2"}, mdOut1.Get("key"))

	outCtx2, conn2, err2 := l.GetConnection(ctx)
	assert.NoError(t, err2)
	assert.Equal(t, conn1, conn2) // connection is cached

	mdOut2, ok2 := metadata.FromOutgoingContext(outCtx2)
	assert.True(t, ok2)
	assert.Equal(t, []string{"value1", "value2"}, mdOut2.Get("key"))
}
