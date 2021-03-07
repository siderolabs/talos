// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/impl/inmem"
	"github.com/talos-systems/os-runtime/pkg/state/impl/namespaced"
	"github.com/talos-systems/os-runtime/pkg/state/registry"

	"github.com/talos-systems/talos/pkg/resources/k8s"
)

func TestRegisterResource(t *testing.T) {
	ctx := context.TODO()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []resource.Resource{
		&k8s.ManifestStatus{},
		&k8s.Manifest{},
		&k8s.SecretsStatus{},
		&k8s.StaticPodStatus{},
		&k8s.StaticPod{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}
