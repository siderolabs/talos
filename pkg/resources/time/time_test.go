// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/resources/time"
)

func TestRegisterResource(t *testing.T) {
	ctx := context.TODO()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []resource.Resource{
		&time.Status{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}
