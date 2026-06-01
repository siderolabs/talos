// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

func TestRegisterResource(t *testing.T) {
	ctx := t.Context()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []meta.ResourceWithRD{
		&cluster.Affiliate{},
		&cluster.Config{},
		&cluster.Identity{},
		&cluster.Member{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}
