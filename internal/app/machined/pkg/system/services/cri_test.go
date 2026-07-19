// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestCreateOverlayMountRequests(t *testing.T) {
	t.Parallel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, services.CreateOverlayMountRequests(t.Context(), st))
	require.NoError(t, services.CreateOverlayMountRequests(t.Context(), st))

	mountRequests, err := safe.StateListAll[*block.VolumeMountRequest](t.Context(), st)
	require.NoError(t, err)
	require.Equal(t, len(constants.Overlays), mountRequests.Len())

	for _, overlay := range constants.Overlays {
		mountRequest, err := safe.StateGetByID[*block.VolumeMountRequest](t.Context(), st, overlay.Path)
		require.NoError(t, err)
		require.Equal(t, "cri", mountRequest.TypedSpec().Requester)
		require.Equal(t, overlay.Path, mountRequest.TypedSpec().VolumeID)
		require.Empty(t, mountRequest.Metadata().Labels().Raw())
	}
}
