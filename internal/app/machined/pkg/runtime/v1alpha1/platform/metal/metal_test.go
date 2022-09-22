// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/talos-systems/talos/pkg/machinery/resources/hardware"
)

func TestNetworkConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	p := &metal.Metal{}

	ch := make(chan *runtime.PlatformNetworkConfig, 1)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	uuid := hardware.NewSystemInformation("test")
	uuid.TypedSpec().UUID = "0123-4567-89ab-cdef"
	require.NoError(t, st.Create(ctx, uuid))

	err := p.NetworkConfiguration(ctx, st, ch)
	require.NoError(t, err)

	select {
	case <-ctx.Done():
		t.Error("timeout")
	case cfg := <-ch:
		assert.Equal(t, "metal", cfg.Metadata.Platform)
		assert.Equal(t, uuid.TypedSpec().UUID, cfg.Metadata.InstanceID)
	}
}
