// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disks_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

func TestObserveCopyVolumeRequiresQualifiedSuccess(t *testing.T) {
	for _, tt := range []struct {
		name         string
		phase        storage.StoragePoolVolumeCopyPhase
		missing      bool
		stale        bool
		retarget     bool
		changeSource bool
		ready        bool
	}{
		{name: "qualified success", phase: storage.StoragePoolVolumeCopyPhaseSucceeded, ready: true},
		{name: "missing observation", missing: true},
		{name: "unknown observation", phase: storage.StoragePoolVolumeCopyPhaseUnknown},
		{name: "running is not success", phase: storage.StoragePoolVolumeCopyPhaseRunning},
		{name: "failed is not success", phase: storage.StoragePoolVolumeCopyPhaseFailed},
		{name: "old intent", phase: storage.StoragePoolVolumeCopyPhaseSucceeded, stale: true},
		{name: "new backing", phase: storage.StoragePoolVolumeCopyPhaseSucceeded, retarget: true},
		{name: "new source dependency", phase: storage.StoragePoolVolumeCopyPhaseSucceeded, changeSource: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := state.WrapCore(namespaced.NewState(inmem.Build))
			request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "images/one.raw")
			*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
				Pool: "images", Name: "one.raw", Format: "raw",
				Copy: &storage.StoragePoolVolumeCopySpec{
					Source: "/library/object", Digest: "sha256:123", SourceLibrary: "library",
				},
			}
			pool := storage.NewStoragePoolStatus(storage.NamespaceName, "images")
			pool.TypedSpec().TargetPath = "/pool"
			pool.TypedSpec().VolumeID = "backing"
			pool.TypedSpec().Phase = storage.StoragePoolPhaseReady

			for _, res := range []resource.Resource{request, pool} {
				require.NoError(t, st.Create(t.Context(), res))
			}

			request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](t.Context(), st, request.Metadata().ID())
			require.NoError(t, err)
			pool, err = safe.StateGetByID[*storage.StoragePoolStatus](t.Context(), st, "images")
			require.NoError(t, err)

			status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, request.Metadata().ID())

			*status.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
				Pool: "images", Name: "one.raw", Format: "raw", Path: "/pool/one.raw",
				Phase: storage.StoragePoolVolumePhaseReady,
				Copy: &storage.StoragePoolVolumeCopyStatusSpec{
					Phase: tt.phase, Intent: storage.StoragePoolVolumeCopyIntent(request, pool),
				},
			}
			if tt.missing {
				status.TypedSpec().Copy = nil
			}

			if tt.stale {
				status.TypedSpec().Copy.Intent = "old-attempt"
			}

			if tt.retarget {
				pool.TypedSpec().TargetPath = "/replacement"
				require.NoError(t, st.Update(t.Context(), pool))
			}

			if tt.changeSource {
				request.TypedSpec().Copy.SourceLibrary = "replacement"
				require.NoError(t, st.Update(t.Context(), request))
			}

			require.NoError(t, st.Create(t.Context(), status))

			observed, err := disks.ObserveVolume(t.Context(), st, request)
			if tt.ready {
				require.NoError(t, err)
				require.Equal(t, hypervisor.VirtualMachineDiskPhaseReady, observed.Phase)
			} else {
				require.Error(t, err)
				require.Equal(t, hypervisor.VirtualMachineDiskPhaseNotReady, observed.Phase)
			}
		})
	}
}
