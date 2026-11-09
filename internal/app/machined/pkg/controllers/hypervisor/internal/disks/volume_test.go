// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disks_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

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
			st := state.WrapCore(inmem.NewState())
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
					Phase: tt.phase, Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
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

func TestObserveWritableCopyAcrossCapacityEdits(t *testing.T) {
	for _, phase := range []storage.StoragePoolVolumeCopyPhase{storage.StoragePoolVolumeCopyPhaseSucceeded, storage.StoragePoolVolumeCopyPhaseRunning} {
		for _, capacity := range []uint64{10 << 30, 40 << 30} {
			t.Run(fmt.Sprintf("%s/%d", phase, capacity), func(t *testing.T) {
				st := state.WrapCore(inmem.NewState())
				request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, "pool/guest.img")
				*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
					Pool: "pool", Name: "guest.img", Format: "qcow2", Capacity: 20 << 30,
					Copy: &storage.StoragePoolVolumeCopySpec{Image: true, Source: "/library/object", Digest: "sha256:123"},
				}
				pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool")

				pool.TypedSpec().TargetPath = "/pool"
				for _, res := range []resource.Resource{request, pool} {
					require.NoError(t, st.Create(t.Context(), res))
				}

				request, err := safe.StateGetByID[*storage.StoragePoolVolumeSpec](t.Context(), st, request.Metadata().ID())
				require.NoError(t, err)
				pool, err = safe.StateGetByID[*storage.StoragePoolStatus](t.Context(), st, pool.Metadata().ID())
				require.NoError(t, err)

				status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, request.Metadata().ID())
				*status.TypedSpec() = storage.StoragePoolVolumeStatusSpec{
					Pool: "pool", Name: "guest.img", Format: "qcow2", Path: "/pool/guest.img", Capacity: 20 << 30,
					Phase: storage.StoragePoolVolumePhaseReady,
					Copy:  &storage.StoragePoolVolumeCopyStatusSpec{Phase: phase, Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool)},
				}
				request.TypedSpec().Capacity = capacity
				require.NoError(t, st.Update(t.Context(), request))
				require.NoError(t, st.Create(t.Context(), status))

				resolved, err := disks.ObserveVolume(t.Context(), st, request)
				if phase == storage.StoragePoolVolumeCopyPhaseSucceeded {
					require.NoError(t, err)
					require.Equal(t, hypervisor.VirtualMachineDiskPhaseReady, resolved.Phase)
					require.EqualValues(t, 20<<30, resolved.Size)
				} else {
					require.Error(t, err)
					require.Equal(t, hypervisor.VirtualMachineDiskPhaseNotReady, resolved.Phase)
				}
			})
		}
	}
}

type failingLibraryHoldWriter struct {
	controller.ReaderWriter
	err error
}

func (writer failingLibraryHoldWriter) AddFinalizer(ctx context.Context, ptr resource.Pointer, finalizers ...resource.Finalizer) error {
	if ptr.Type() == hypervisor.ContentLibraryStatusType && len(finalizers) == 1 && finalizers[0] == disks.ControllerName {
		return writer.err
	}

	return writer.ReaderWriter.AddFinalizer(ctx, ptr, finalizers...)
}

func TestMalformedBindingAndHoldFailureJoinErrors(t *testing.T) {
	r, request, library := malformedCopyBindingFixture(t)
	holdErr := errors.New("injected library hold failure")

	libraries := map[resource.ID]*hypervisor.ContentLibraryStatus{
		library.Metadata().ID(): library,
	}
	_, err := disks.RetainCopySource(t.Context(), failingLibraryHoldWriter{ReaderWriter: r, err: holdErr}, zap.NewNop(), request, libraries)
	require.ErrorContains(t, err, "invalid copy binding record")
	require.ErrorIs(t, err, holdErr)
}

func TestSucceededCopyDoesNotReadMalformedBindingOrHoldSource(t *testing.T) {
	r, request, library := malformedCopyBindingFixture(t)
	pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](t.Context(), r, "pool")
	require.NoError(t, err)

	status := storage.NewStoragePoolVolumeStatus(storage.NamespaceName, request.Metadata().ID())
	status.TypedSpec().Phase = storage.StoragePoolVolumePhaseReady
	status.TypedSpec().Pool = request.TypedSpec().Pool
	status.TypedSpec().Name = request.TypedSpec().Name
	status.TypedSpec().Format = request.TypedSpec().Format
	status.TypedSpec().Path = filepath.Join(pool.TypedSpec().TargetPath, request.TypedSpec().Name)
	status.TypedSpec().Copy = &storage.StoragePoolVolumeCopyStatusSpec{
		Phase:  storage.StoragePoolVolumeCopyPhaseSucceeded,
		Intent: storage.StoragePoolVolumeCopyInitializationIntent(request, pool),
	}
	require.NoError(t, r.Create(t.Context(), status))

	held, err := disks.RetainCopySource(t.Context(), r, zap.NewNop(), request, map[resource.ID]*hypervisor.ContentLibraryStatus{library.Metadata().ID(): library})
	require.NoError(t, err)
	require.Empty(t, held)
	current, err := safe.ReaderGetByID[*hypervisor.ContentLibraryStatus](t.Context(), r, library.Metadata().ID())
	require.NoError(t, err)
	require.False(t, current.Metadata().Finalizers().Has(disks.ControllerName))
}
