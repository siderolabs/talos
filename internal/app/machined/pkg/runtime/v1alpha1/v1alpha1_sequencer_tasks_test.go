// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestUnmountEphemeralPartitionWaitsForFinalizers(t *testing.T) {
	t.Setenv("PLATFORM", "container")

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	runtimeState, err := v1alpha1.NewState()
	require.NoError(t, err)

	resources := runtimeState.V1Alpha2().Resources()
	mountRequest := blockres.NewVolumeMountRequest(blockres.NamespaceName, constants.EphemeralPartitionLabel)

	require.NoError(t, resources.Create(ctx, mountRequest))
	require.NoError(t, resources.AddFinalizer(ctx, mountRequest.Metadata(), "mount-controller"))

	finalizerRemoved := make(chan error, 1)

	go func() {
		_, watchErr := resources.WatchFor(ctx, mountRequest.Metadata(), state.WithPhases(resource.PhaseTearingDown))
		if watchErr != nil {
			finalizerRemoved <- watchErr

			return
		}

		finalizerRemoved <- resources.RemoveFinalizer(ctx, mountRequest.Metadata(), "mount-controller")
	}()

	task, _ := v1alpha1.UnmountEphemeralPartition(runtime.SequenceReset, nil)
	err = task(ctx, log.New(io.Discard, "", 0), v1alpha1.NewRuntime(runtimeState, nil, nil))

	require.NoError(t, err)
	require.NoError(t, <-finalizerRemoved)

	_, err = resources.Get(ctx, mountRequest.Metadata())
	require.True(t, state.IsNotFoundError(err))
}
