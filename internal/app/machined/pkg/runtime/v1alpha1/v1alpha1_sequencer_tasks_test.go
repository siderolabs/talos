// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package v1alpha1

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestResolveStagedVolumeWipeTargets(t *testing.T) {
	ctx := t.Context()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	dv := blockres.NewDiscoveredVolume(blockres.NamespaceName, "vda3")
	dv.TypedSpec().PartitionLabel = "EPHEMERAL"
	dv.TypedSpec().DevPath = "/dev/vda3"
	dv.TypedSpec().ParentDevPath = "/dev/vda"
	dv.TypedSpec().PartitionIndex = 3
	require.NoError(t, st.Create(ctx, dv))

	targets, missing, err := resolveStagedVolumeWipeTargets(ctx, st, []string{"EPHEMERAL", "STATE"})
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.Equal(t, "EPHEMERAL", targets[0].GetLabel())
	assert.Equal(t, []string{"STATE"}, missing)
}
