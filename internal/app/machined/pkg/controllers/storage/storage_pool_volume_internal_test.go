// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// heldPoolRace changes the pool exactly when the hold is acquired, before the
// mandatory fresh read. No scheduler timing or daemon outage sleep is involved.
type heldPoolRace struct {
	controller.ReaderWriter
	pool   *storageres.StoragePoolStatus
	change func(*storageres.StoragePoolStatus)
	reads  int
	held   bool
}

func (r *heldPoolRace) AddFinalizer(_ context.Context, _ resource.Pointer, finalizers ...resource.Finalizer) error {
	r.held = true
	for _, finalizer := range finalizers {
		r.pool.Metadata().Finalizers().Add(finalizer)
	}

	r.change(r.pool)

	return nil
}

func (r *heldPoolRace) Get(context.Context, resource.Pointer, ...state.GetOption) (resource.Resource, error) {
	r.reads++

	return r.pool, nil
}

func TestVolumePoolHoldObservationRace(t *testing.T) {
	for _, tt := range []struct {
		name        string
		change      func(*storageres.StoragePoolStatus)
		unavailable bool
	}{
		{name: "same backing outage", unavailable: true, change: func(*storageres.StoragePoolStatus) {}},
		{name: "withdrawn", change: func(p *storageres.StoragePoolStatus) { p.Metadata().SetPhase(resource.PhaseTearingDown) }},
		{name: "retargeted volume", change: func(p *storageres.StoragePoolStatus) { p.TypedSpec().VolumeID = "replacement" }},
		{name: "retargeted directory", change: func(p *storageres.StoragePoolStatus) { p.TypedSpec().TargetPath = "/replacement" }},
		{name: "recreated", change: func(p *storageres.StoragePoolStatus) {
			p.Metadata().SetCreated(p.Metadata().Created().Add(time.Second))
		}},
		{name: "pool mutation", change: func(p *storageres.StoragePoolStatus) {
			p.Metadata().Finalizers().Add("storage.StoragePoolController/mutating/backing")
		}},
		{name: "uncertain volume mutation", change: func(p *storageres.StoragePoolStatus) {
			p.Metadata().Finalizers().Add(storageres.StoragePoolVolumeMutationFinalizer("guest.raw"))
		}},
		{name: "ordinary not ready", change: func(p *storageres.StoragePoolStatus) { p.TypedSpec().Phase = storageres.StoragePoolPhaseNotReady }},
	} {
		for _, initialUnavailable := range []bool{false, true} {
			name := tt.name + "/ready snapshot"
			if initialUnavailable {
				name = tt.name + "/unavailable snapshot"
			}

			t.Run(name, func(t *testing.T) {
				pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")

				*pool.TypedSpec() = storageres.StoragePoolStatusSpec{VolumeID: "backing", TargetPath: "/pool", Phase: storageres.StoragePoolPhaseReady}
				if initialUnavailable {
					pool.TypedSpec().Phase = storageres.StoragePoolPhaseObservationUnavailable
				}

				r := &heldPoolRace{pool: pool.DeepCopy().(*storageres.StoragePoolStatus), change: func(p *storageres.StoragePoolStatus) {
					p.TypedSpec().Phase = storageres.StoragePoolPhaseObservationUnavailable
					tt.change(p)
				}}
				if initialUnavailable {
					r.change(r.pool)
				}

				ctrl := &StoragePoolVolumeController{copies: &copyWork{}}
				status, err := ctrl.resolve(t.Context(), r, zap.NewNop(),
					&storageres.StoragePoolVolumeSpecSpec{Pool: "images", Name: "guest.raw", Format: "raw"},
					map[string]*storageres.StoragePoolStatus{"images": pool}, map[string]struct{}{},
					func() (libvirtstorage.VolumeClient, error) {
						t.Fatal("must not perform daemon work after pool changed")

						return nil, nil
					}, &volumeMutation{})
				require.Error(t, err)
				require.Equal(t, !initialUnavailable, r.held)
				require.Positive(t, r.reads, "must re-read after acquiring the hold")

				phase := storageres.StoragePoolVolumePhaseNotReady
				if tt.unavailable {
					phase = storageres.StoragePoolVolumePhaseObservationUnavailable
				}

				require.Equal(t, phase, status.Phase, "%v", err)
				require.Equal(t, tt.unavailable, errors.Is(err, libvirtstorage.ErrVolumeObservationUnavailable))
			})
		}
	}
}

func TestVolumePoolObservationRecoveryRace(t *testing.T) {
	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	*pool.TypedSpec() = storageres.StoragePoolStatusSpec{VolumeID: "backing", TargetPath: "/pool", Phase: storageres.StoragePoolPhaseObservationUnavailable}
	r := &heldPoolRace{pool: pool.DeepCopy().(*storageres.StoragePoolStatus)}
	r.pool.TypedSpec().Phase = storageres.StoragePoolPhaseReady

	// Recovery between the snapshot and fresh read must not publish NotReady
	// either. The queued pool event will permit work on the next pass.
	ctrl := &StoragePoolVolumeController{copies: &copyWork{}}
	status, err := ctrl.resolve(t.Context(), r, zap.NewNop(),
		&storageres.StoragePoolVolumeSpecSpec{Pool: "images", Name: "guest.raw", Format: "raw"},
		map[string]*storageres.StoragePoolStatus{"images": pool}, map[string]struct{}{},
		func() (libvirtstorage.VolumeClient, error) {
			t.Fatal("defer daemon work until the recovery event")

			return nil, nil
		}, &volumeMutation{})
	require.ErrorIs(t, err, libvirtstorage.ErrVolumeObservationUnavailable)
	require.Equal(t, storageres.StoragePoolVolumePhaseObservationUnavailable, status.Phase)
	require.Positive(t, r.reads)
}

func TestCopyProgressDebugThrottleAndBoundaries(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	observe := copyProgressObserver(zap.New(core))
	require.NotNil(t, observe)

	for _, progress := range []libvirtstorage.CopyProgress{
		{Phase: libvirtstorage.CopyPhaseTransfer, TotalKnown: true, TotalBytes: 10},
		{Phase: libvirtstorage.CopyPhaseTransfer, BytesCopied: 1, Elapsed: time.Second, TotalKnown: true, TotalBytes: 10},
		{Phase: libvirtstorage.CopyPhaseTransfer, BytesCopied: 4, Elapsed: 4 * time.Second, TotalKnown: true, TotalBytes: 10},
		{Phase: libvirtstorage.CopyPhaseTransfer, BytesCopied: 5, Elapsed: 5 * time.Second, TotalKnown: true, TotalBytes: 10},
		{Phase: libvirtstorage.CopyPhaseVerify, BytesCopied: 10, Elapsed: 5100 * time.Millisecond, TotalKnown: true, TotalBytes: 10},
		{Phase: libvirtstorage.CopyPhaseSync, BytesCopied: 10, Elapsed: 5200 * time.Millisecond, TotalKnown: true, TotalBytes: 10},
		{Phase: libvirtstorage.CopyPhasePublish, BytesCopied: 10, Elapsed: 5300 * time.Millisecond, TotalKnown: true, TotalBytes: 10},
		{Phase: libvirtstorage.CopyPhaseCleanup, BytesCopied: 10, Elapsed: 5400 * time.Millisecond, TotalKnown: true, TotalBytes: 10},
	} {
		observe(progress)
	}

	entries := logs.All()
	require.Len(t, entries, 6, "throttle transfer chunks, never lifecycle boundaries")
	require.Equal(t, uint64(5), entries[1].ContextMap()["copied_bytes"])
	require.Equal(t, float64(1), entries[1].ContextMap()["bytes_per_second"])
	require.Equal(t, uint64(10), entries[1].ContextMap()["total_bytes"])

	for i, phase := range []libvirtstorage.CopyPhase{
		libvirtstorage.CopyPhaseTransfer,
		libvirtstorage.CopyPhaseTransfer,
		libvirtstorage.CopyPhaseVerify,
		libvirtstorage.CopyPhaseSync,
		libvirtstorage.CopyPhasePublish,
		libvirtstorage.CopyPhaseCleanup,
	} {
		require.Equal(t, string(phase), entries[i].ContextMap()["copy_phase"])
		require.Equal(t, zap.DebugLevel, entries[i].Level)
	}
}

func TestCopyProgressDebugUnknownAndEmptyTotals(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	observe := copyProgressObserver(zap.New(core))
	observe(libvirtstorage.CopyProgress{Phase: libvirtstorage.CopyPhaseTransfer})
	observe(libvirtstorage.CopyProgress{Phase: libvirtstorage.CopyPhaseCleanup, TotalKnown: true})

	entries := logs.All()
	require.Len(t, entries, 2)
	require.NotContains(t, entries[0].ContextMap(), "total_bytes")
	require.Equal(t, uint64(0), entries[1].ContextMap()["total_bytes"])
}

func TestCopyProgressObserverDisabled(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)

	require.Nil(t, copyProgressObserver(nil))
	require.Nil(t, copyProgressObserver(zap.New(core)))
	require.Nil(t, copyProgressObserver(zap.NewNop()))
	require.Empty(t, logs.All())
}

func TestCopyFailureBackoffPersistsAcrossReconciliation(t *testing.T) {
	now := time.Unix(100, 0)
	failure := errors.New("qemu-img convert: no space left on device")

	proof := &copyProof{intent: "request/backing"}

	for attempt, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, time.Minute, time.Minute} {
		proof.recordAttempt(failure, false, now)
		require.Equal(t, want, proof.retryAt.Sub(now), "attempt %d", attempt)
		err := proof.attemptErrorAt(now)
		require.ErrorIs(t, err, failure)
		require.Equal(t, want, volumeRetryDelay([]error{err}, now))
		// A status write or input event must not restart the clock or admit work early.
		require.Equal(t, want/2, volumeRetryDelay([]error{proof.attemptErrorAt(now.Add(want / 2))}, now.Add(want/2)))
		now = now.Add(want)
		require.NoError(t, proof.attemptErrorAt(now))
	}

	proof.recordAttempt(nil, false, now)
	proof.recordAttempt(failure, false, now)
	require.Equal(t, time.Second, proof.retryAt.Sub(now), "success resets backoff")
	work := &copyWork{proofs: map[resource.ID]*copyProof{"volume": proof}}
	require.Same(t, proof, work.proofFor("volume", "request/backing", true))
	require.NotSame(t, proof, work.proofFor("volume", "new request/backing", true), "identity change resets backoff")
	require.Equal(t, time.Second, volumeRetryDelay([]error{failure, proof.attemptErrorAt(now)}, now), "other failures retain ordinary retry")
}

func TestCopyTerminalAttemptDoesNotScheduleRetry(t *testing.T) {
	proof := &copyProof{}
	proof.recordAttempt(errors.New("invalid image format"), true, time.Unix(100, 0))
	require.ErrorIs(t, proof.attemptErrorAt(time.Unix(1000, 0)), errVolumePending)
	require.Zero(t, volumeRetryDelay(nil, time.Unix(1000, 0)))
}

// Lose observation after successful daemon resolution, at binding qualification.
type qualificationPoolRace struct {
	controller.Writer
	state.StateReader
	beforePoolRead func()
	poolReadErr    error
}

func (r *qualificationPoolRace) Get(ctx context.Context, ptr resource.Pointer, options ...state.GetOption) (resource.Resource, error) {
	if ptr.Type() == storageres.StoragePoolStatusType && r.beforePoolRead != nil {
		change := r.beforePoolRead
		r.beforePoolRead = nil

		change()
	}

	if ptr.Type() == storageres.StoragePoolStatusType && r.poolReadErr != nil {
		return nil, r.poolReadErr
	}

	return r.StateReader.Get(ctx, ptr, options...)
}

func TestPendingResolutionDoesNotMaskCopyQualificationFailure(t *testing.T) {
	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "pool")
	*pool.TypedSpec() = storageres.StoragePoolStatusSpec{
		Phase:      storageres.StoragePoolPhaseReady,
		TargetPath: "/pool",
		VolumeID:   "backing",
	}
	require.NoError(t, st.Create(ctx, pool))

	spec := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, "pool/guest.img")
	*spec.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
		Pool:     "pool",
		Name:     "guest.img",
		Format:   "raw",
		Capacity: 1024,
		Copy:     &storageres.StoragePoolVolumeCopySpec{Image: true},
	}
	require.NoError(t, st.Create(ctx, spec))

	failure := errors.New("qualification pool read failed")
	r := &qualificationPoolRace{
		StateReader: st,
		poolReadErr: failure,
	}
	ctrl := &StoragePoolVolumeController{copies: &copyWork{}}
	resolved, err := ctrl.observeCopy(ctx, r, spec, map[string]*storageres.StoragePoolStatus{"pool": pool},
		storageres.StoragePoolVolumeStatusSpec{
			Phase:           storageres.StoragePoolVolumePhaseReady,
			PendingCapacity: 1024,
		},
		pending("growth deferred while guest attached"))
	require.ErrorIs(t, err, errVolumePending)
	require.ErrorIs(t, err, failure)
	require.True(t, retryableVolumeResolution(err), "real qualification failure must requeue despite pending growth")
	require.Equal(t, storageres.StoragePoolVolumePhaseNotReady, resolved.Phase)
}

func TestCompletedCopyQualificationPoolObservationRace(t *testing.T) {
	ctx := t.Context()
	st := state.WrapCore(inmem.NewState())
	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "pool")
	*pool.TypedSpec() = storageres.StoragePoolStatusSpec{
		Phase:      storageres.StoragePoolPhaseReady,
		TargetPath: "/pool",
		VolumeID:   "backing",
	}
	require.NoError(t, st.Create(ctx, pool))

	request := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, "pool/guest.img")
	*request.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
		Pool:     "pool",
		Name:     "guest.img",
		Format:   "raw",
		Capacity: 1024,
		Copy:     &storageres.StoragePoolVolumeCopySpec{Image: true},
	}
	request.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, "binding")
	require.NoError(t, st.Create(ctx, request, state.WithCreateOwner("hypervisor.VirtualMachineDiskController")))

	disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, "vm/system")
	disk.TypedSpec().Pool, disk.TypedSpec().Volume = "pool", "guest.img"
	disk.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, "binding")
	require.NoError(t, st.Create(ctx, disk, state.WithCreateOwner("hypervisor.VirtualMachineDiskController")))
	request, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](ctx, st, request.Metadata().ID())
	require.NoError(t, err)
	pool, err = safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, "pool")
	require.NoError(t, err)

	previous := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, request.Metadata().ID())
	*previous.TypedSpec() = storageres.StoragePoolVolumeStatusSpec{
		Phase: storageres.StoragePoolVolumePhaseReady,
		Copy: &storageres.StoragePoolVolumeCopyStatusSpec{
			Phase:  storageres.StoragePoolVolumeCopyPhaseSucceeded,
			Intent: storageres.StoragePoolVolumeCopyInitializationIntent(request, pool),
		},
	}
	require.NoError(t, st.Create(ctx, previous, state.WithCreateOwner(volumeControllerName)))
	r := &qualificationPoolRace{StateReader: st, beforePoolRead: func() {
		current := pool.DeepCopy().(*storageres.StoragePoolStatus)
		current.TypedSpec().Phase = storageres.StoragePoolPhaseObservationUnavailable
		require.NoError(t, st.Update(ctx, current))
	}}
	ctrl := &StoragePoolVolumeController{copies: &copyWork{}}
	resolved, err := ctrl.observeCopy(ctx, r, request, map[string]*storageres.StoragePoolStatus{"pool": pool},
		storageres.StoragePoolVolumeStatusSpec{Phase: storageres.StoragePoolVolumePhaseReady}, nil)
	require.ErrorIs(t, err, libvirtstorage.ErrVolumeObservationUnavailable)
	require.Equal(t, storageres.StoragePoolVolumePhaseObservationUnavailable, resolved.Phase)
	require.Equal(t, previous.TypedSpec().Copy, resolved.Copy, "qualification outage must not erase completed initialization")
}
