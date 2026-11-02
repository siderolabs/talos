// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestExecutionHashCache(t *testing.T) {
	t.Parallel()

	var cache executionHashCache

	spec := containers.NewContainerSpec(containers.NamespaceName, "app")
	spec.TypedSpec().Image.Ref = "ghcr.io/example/app:v1"
	spec.Metadata().SetVersion(spec.Metadata().Version().Next())

	hash := cache.get(spec)
	assert.Equal(t, spec.TypedSpec().ExecutionHash(), hash)

	// the same version is not hashed again, even if its contents differ (which a stored resource's can't)
	spec.TypedSpec().Image.Ref = "ghcr.io/example/app:v2"
	assert.Equal(t, hash, cache.get(spec))

	// a new version is
	spec.Metadata().SetVersion(spec.Metadata().Version().Next())
	assert.Equal(t, spec.TypedSpec().ExecutionHash(), cache.get(spec))
	assert.NotEqual(t, hash, cache.get(spec))

	// a spec created again starts over with its version
	recreated := containers.NewContainerSpec(containers.NamespaceName, "app")
	recreated.TypedSpec().Image.Ref = "ghcr.io/example/app:v3"
	recreated.Metadata().SetVersion(spec.Metadata().Version())
	recreated.Metadata().SetCreated(time.Now().Add(time.Second))
	assert.Equal(t, recreated.TypedSpec().ExecutionHash(), cache.get(recreated))

	cache.retain(map[string]struct{}{})
	assert.Empty(t, cache)
}
