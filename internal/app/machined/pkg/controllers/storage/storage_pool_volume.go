// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/cleanup"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage/internal/mutationevidence"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/libvirt"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
	"github.com/siderolabs/talos/internal/pkg/qemuimg"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const volumeControllerName = "storage.StoragePoolVolumeController"

// copyMaxRunning is node-wide, including workers still draining cancellation.
const copyMaxRunning = 1

// errVolumePending marks a condition an input event resolves, rather than restart backoff.
var errVolumePending = errors.New("volume pending")

var errResizeUncertain = errors.New("resize outcome uncertain")

// pendingError carries errVolumePending without saying so.
//
// The condition is reported through the volume's status, which an operator reads: prefixing it with
// the sentinel's own text would say "volume pending" of conditions which are not pending at all,
// such as a volume whose format differs from the one asked for.
type pendingError struct{ err error }

// copyQualificationError distinguishes a failed independent qualification from a pending
// volume condition when both are reported in the same status.
type copyQualificationError struct{ error }

func (e copyQualificationError) Unwrap() error {
	return e.error
}

func retryableVolumeResolution(err error) bool {
	qualification, qualified := errors.AsType[copyQualificationError](err)

	return err != nil && (!errors.Is(err, errVolumePending) || (qualified && qualification.error != nil))
}

func (e pendingError) Error() string   { return e.err.Error() }
func (e pendingError) Unwrap() []error { return []error{errVolumePending, e.err} }

// pending marks a condition the status reports and an input event resolves, rather than one restart
// backoff should retry.
func pending(format string, args ...any) error { return pendingError{fmt.Errorf(format, args...)} }

// StoragePoolVolumeController makes the volumes asked for exist in their storage pools.
//
// It is the only writer of volumes, as StoragePoolController is the only writer of pools: both
// speak to the same storage daemon, and a pool being redefined underneath a volume being created is
// not something two controllers could agree on after the fact.
//
// It never deletes a volume. A spec going away means nothing is asking for the volume any more, not
// that its contents may go: the configuration which asked for a disk is not the owner of what a
// guest then wrote into it.
type StoragePoolVolumeController struct {
	V1Alpha1Mode machineruntime.Mode

	// Open is injectable for deterministic reconciliation tests.
	Open func(context.Context) (libvirtstorage.VolumeClient, error)
	// ImageTool selects qemu-img for image import tests; nil uses the runtime extension.
	ImageTool *qemuimg.Client

	bindings copyowner.Bindings
	copies   *copyWork

	privateCopyNow         func() time.Time
	nextPrivateCopyCleanup time.Time
	privateCopyConfigID    string
}

// Name implements controller.Controller interface.
func (ctrl *StoragePoolVolumeController) Name() string {
	return volumeControllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *StoragePoolVolumeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeSpecType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.SystemInformationType,
			ID:        optional.Some(hardware.SystemInformationID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StoragePoolVolumeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.StoragePoolVolumeStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
// Reconciliation wakes on input events and copy completions. Failed volume operations
// retry within this Run, without canceling unrelated copies. State failures still
// use runtime restart backoff. Private artifacts are scanned at most once a minute
// for unchanged canonical configuration. Each sweep holds and rechecks its backing.
func (ctrl *StoragePoolVolumeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	ctrl.bindings = copyowner.Bindings{}

	ctrl.copies = ctrl.startCopies(ctx)

	ctrl.copies.logger = logger
	defer func() {
		ctrl.copies.stop()
		ctrl.copies = nil
	}()

	retry := time.NewTimer(time.Second)

	retry.Stop()
	defer retry.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case job := <-ctrl.copies.finished:
			ctrl.copies.complete(job)
		case <-r.EventCh():
		case <-retry.C:
		}

		specs, err := safe.ReaderListAll[*storage.StoragePoolVolumeSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing storage pool volume specs: %w", err)
		}

		delay, err := ctrl.reconcile(ctx, r, logger, specs)
		if err != nil {
			return err
		}

		if delay > 0 {
			retry.Reset(delay)
		} else {
			retry.Stop()
		}

		r.ResetRestartBackoff()
	}
}

// reconcile brings every asked-for volume into being, opening a session only if one is needed.
//
//nolint:gocyclo // Keep hold acquisition, output withdrawal and cleanup acknowledgements in order.
func (ctrl *StoragePoolVolumeController) reconcile(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, specs safe.List[*storage.StoragePoolVolumeSpec],
) (time.Duration, error) {
	if err := cleanupCopyBindingFences(ctx, r); err != nil {
		return 0, err
	}

	poolStatuses, err := volumePoolStatuses(ctx, r)
	if err != nil {
		return 0, err
	}

	ctrl.copies.fence(specs, poolStatuses)

	_, poolsInUse, err := volumesInUse(ctx, r)
	if err != nil {
		return 0, err
	}

	// Seeded with the pools a guest already has a volume of open: those stay held whether or not a
	// spec resolves against them this pass, and in particular while one is tearing down.
	held := maps.Clone(poolsInUse)
	// Cancellation requests are not cleanup acknowledgements. Keep the old backing held
	// until the worker has removed its stage and closed the destination root.
	for _, job := range ctrl.copies.jobs {
		held[job.spec.Pool] = struct{}{}
	}

	var (
		session libvirtstorage.VolumeClient
		errs    []error
	)

	defer func() {
		if session != nil {
			session.Close()
		}
	}()

	// Opened lazily, and at most once: a host with no volumes to reconcile must not dial the daemon
	// at all, or every pass turns a daemon outage into a restart loop for nothing.
	openSession := func() (libvirtstorage.VolumeClient, error) {
		if session != nil {
			return session, nil
		}

		opened, openErr := ctrl.Open(ctx)
		if openErr != nil {
			return nil, fmt.Errorf("waiting for storage daemon: %w", openErr)
		}

		session = opened

		return session, nil
	}

	wanted, specErrs, err := ctrl.reconcileSpecs(ctx, r, logger, specs, poolStatuses, held, openSession)

	errs = append(errs, specErrs...)

	if err != nil {
		return 0, errors.Join(append(errs, err)...)
	}

	_, cleanupErr := ctrl.cleanupPrivateCopies(ctx, r, logger, poolStatuses, held)
	if cleanupErr != nil {
		if !errors.Is(cleanupErr, errVolumePending) {
			return 0, errors.Join(append(errs, cleanupErr)...)
		}

		logger.Debug("private copy maintenance deferred", zap.Error(cleanupErr))
	}

	// A status nothing asks for any more is torn down rather than destroyed, so whatever holds it
	// has a chance to notice and give the hold back. The volume's file is untouched either way.
	if err = cleanup.Outputs[*storage.StoragePoolVolumeStatus](ctx, r, "storage pool volume status", wanted); err != nil {
		return 0, errors.Join(append(errs, err)...)
	}

	if err = ctrl.releaseSpecs(ctx, r, specs, wanted); err != nil {
		return 0, errors.Join(append(errs, err)...)
	}

	if len(specErrs) > 0 {
		logger.Debug("retrying failed volume operations without interrupting active copies", zap.Errors("errors", specErrs))
	}

	now := time.Now()

	return privateCopyRetryDelay(volumeRetryDelay(specErrs, now), ctrl.nextPrivateCopyCleanup, now), ctrl.releasePools(ctx, r, logger, poolStatuses, held)
}

// volumePoolStatuses observes pools, including those held by an uncertain resize.
func volumePoolStatuses(ctx context.Context, r controller.Reader) (map[string]*storage.StoragePoolStatus, error) {
	pools, err := safe.ReaderListAll[*storage.StoragePoolStatus](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing storage pool statuses: %w", err)
	}

	poolStatuses := make(map[string]*storage.StoragePoolStatus, pools.Len())
	for pool := range pools.All() {
		poolStatuses[pool.Metadata().ID()] = pool
	}

	return poolStatuses, nil
}

// reconcileSpecs publishes the status of every volume still asked for, and names them.
//
// A per-volume failure is collected rather than returned: one unreachable volume must not stop the
// rest from being reported. Only a failure to write a status at all is returned.
func (ctrl *StoragePoolVolumeController) reconcileSpecs(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	specs safe.List[*storage.StoragePoolVolumeSpec],
	poolStatuses map[string]*storage.StoragePoolStatus,
	held map[string]struct{},
	openSession func() (libvirtstorage.VolumeClient, error),
) (map[resource.ID]struct{}, []error, error) {
	wanted := map[resource.ID]struct{}{}

	var errs []error

	ordered := slices.Collect(specs.All())
	slices.SortFunc(ordered, func(a, b *storage.StoragePoolVolumeSpec) int {
		if order := compareCopyCursor(a.TypedSpec().Pool, b.TypedSpec().Pool, ctrl.copies.cursor.pool); order != 0 {
			return order
		}

		return compareCopyCursor(a.Metadata().ID(), b.Metadata().ID(), ctrl.copies.cursor.requests[a.TypedSpec().Pool])
	})

	for _, spec := range ordered {
		if spec.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		id := spec.Metadata().ID()
		wanted[id] = struct{}{}

		withdrawing, specErrs, err := ctrl.reconcileVolumeSpec(ctx, r, logger, spec, poolStatuses, held, openSession)

		errs = append(errs, specErrs...)
		if err != nil {
			return wanted, errs, err
		}

		if withdrawing {
			delete(wanted, id)
		}
	}

	return wanted, errs, nil
}

// reconcileVolumeSpec preserves per-volume error collection while keeping the
// withdrawal, hold, mutation and status-publication order in one place.
func (ctrl *StoragePoolVolumeController) reconcileVolumeSpec(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	spec *storage.StoragePoolVolumeSpec, poolStatuses map[string]*storage.StoragePoolStatus,
	held map[string]struct{}, openSession func() (libvirtstorage.VolumeClient, error),
) (bool, []error, error) {
	id := spec.Metadata().ID()

	withdrawing, err := volumeStatusWithdrawing(ctx, r, spec)
	if err != nil || withdrawing {
		return withdrawing, nil, err
	}

	// Never revive or mutate a prior request before its consumers detach.
	var errs []error

	if holdErr := ctrl.holdSpec(ctx, r, spec); holdErr != nil {
		errs = append(errs, holdErr)

		return false, errs, nil
	}

	var mutation volumeMutation

	resolved, resolveErr := ctrl.resolve(ctx, r, logger, spec.TypedSpec(), poolStatuses, held, openSession, &mutation)

	resolved, resolveErr = ctrl.observeCopy(ctx, r, spec, poolStatuses, resolved, resolveErr)

	if retryableVolumeResolution(resolveErr) {
		// A libvirt or filesystem failure emits no resource event: retry via backoff.
		errs = append(errs, fmt.Errorf("volume %q: %w", id, resolveErr))
	}

	releaseErr, publishErr := publishVolumeResolution(ctx, r, spec, resolved, resolveErr, &mutation, logger)
	if publishErr != nil {
		return false, errs, fmt.Errorf("error updating storage pool volume status %q: %w", id, publishErr)
	}

	if releaseErr != nil {
		errs = append(errs, fmt.Errorf("volume %q: release mutation marker: %w", id, releaseErr))
	}

	return false, errs, nil
}

// observeCopy separates copy outcome from volume attachability.
func (ctrl *StoragePoolVolumeController) observeCopy(ctx context.Context, r controller.ReaderWriter,
	spec *storage.StoragePoolVolumeSpec, poolStatuses map[string]*storage.StoragePoolStatus,
	resolved storage.StoragePoolVolumeStatusSpec, resolveErr error,
) (storage.StoragePoolVolumeStatusSpec, error) {
	if resolved.Phase == storage.StoragePoolVolumePhaseReady && spec.TypedSpec().Copy != nil {
		if err := ctrl.qualifyCopy(ctx, r, spec, poolStatuses); err != nil {
			if !errors.Is(err, errVolumePending) {
				err = copyQualificationError{err}
			}

			resolveErr = errors.Join(resolveErr, err)

			resolved.Phase = storage.StoragePoolVolumePhaseNotReady
			if errors.Is(err, libvirtstorage.ErrVolumeObservationUnavailable) {
				resolved.Phase = storage.StoragePoolVolumePhaseObservationUnavailable
			}
		}
	}

	if spec.TypedSpec().Copy != nil {
		resolved.Copy, resolveErr = ctrl.observeCopyStatus(ctx, r, spec, poolStatuses[spec.TypedSpec().Pool], resolved, resolveErr)
	}

	return resolved, resolveErr
}

func (ctrl *StoragePoolVolumeController) observeCopyStatus(ctx context.Context, r controller.Reader,
	spec *storage.StoragePoolVolumeSpec, pool *storage.StoragePoolStatus,
	resolved storage.StoragePoolVolumeStatusSpec, resolveErr error,
) (*storage.StoragePoolVolumeCopyStatusSpec, error) {
	copyStatus := resolved.Copy
	if copyStatus == nil {
		copyStatus = &storage.StoragePoolVolumeCopyStatusSpec{Phase: storage.StoragePoolVolumeCopyPhaseWaiting}
	}

	copyStatus.Intent = storage.StoragePoolVolumeCopyInitializationIntent(spec, pool)

	if job := ctrl.copies.jobs[spec.Metadata().ID()]; job != nil {
		copyStatus.Phase = job.phase()
	} else if resolved.Phase == storage.StoragePoolVolumePhaseReady {
		copyStatus.Phase = storage.StoragePoolVolumeCopyPhaseSucceeded
	} else if resolved.Phase == storage.StoragePoolVolumePhaseObservationUnavailable {
		previous, err := retainedCopyCompletion(ctx, r, spec, pool)
		if previous != nil {
			copyStatus = previous
		}

		resolveErr = errors.Join(resolveErr, err)
	}

	return copyStatus, resolveErr
}

// retainedCopyCompletion preserves qualified history, never attachability. An active
// job takes precedence above; withdrawal, mutation and changed binding intent revoke
// retention even when the daemon cannot be reached to observe the copied bytes.
func retainedCopyCompletion(ctx context.Context, r controller.Reader, spec *storage.StoragePoolVolumeSpec,
	pool *storage.StoragePoolStatus,
) (*storage.StoragePoolVolumeCopyStatusSpec, error) {
	if pool == nil {
		return nil, nil
	}

	bound, err := retainedCopyRequestCurrent(ctx, r, spec)
	if err != nil || !bound {
		return nil, err
	}

	if err = recheckHeldPool(ctx, r, spec.TypedSpec().Pool, spec.TypedSpec().Name, pool); err != nil &&
		!errors.Is(err, libvirtstorage.ErrVolumeObservationUnavailable) {
		return nil, nil
	}

	previous, err := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, r, spec.Metadata().ID())
	if state.IsNotFoundError(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if !copyCompletionMatches(previous, spec, pool) {
		return nil, nil
	}

	return previous.TypedSpec().Copy, nil
}

func copyCompletionMatches(previous *storage.StoragePoolVolumeStatus, spec *storage.StoragePoolVolumeSpec, pool *storage.StoragePoolStatus) bool {
	copyStatus := previous.TypedSpec().Copy

	return previous.Metadata().Phase() == resource.PhaseRunning && previous.Metadata().Owner() == volumeControllerName &&
		(previous.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady || previous.TypedSpec().Phase == storage.StoragePoolVolumePhaseObservationUnavailable) &&
		storage.StoragePoolVolumeCopyCompletionMatches(spec, pool, copyStatus)
}

func retainedCopyRequestCurrent(ctx context.Context, r controller.Reader, spec *storage.StoragePoolVolumeSpec) (bool, error) {
	current, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, spec.Metadata().ID())
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if !sameCopyRequest(current, spec) || current.Metadata().Owner() != "hypervisor.VirtualMachineDiskController" {
		return false, nil
	}

	key, _ := current.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	if key == "" {
		return false, nil
	}

	intent, err := copyBindingIntent(ctx, r, key, current.TypedSpec())

	return intent != nil, err
}

// qualifyCopy requires current request/backing and durable binding before success.
func (ctrl *StoragePoolVolumeController) qualifyCopy(ctx context.Context, r controller.ReaderWriter,
	spec *storage.StoragePoolVolumeSpec, poolStatuses map[string]*storage.StoragePoolStatus,
) error {
	observation := &copyJob{
		id:     spec.Metadata().ID(),
		spec:   *spec.TypedSpec(),
		intent: storage.StoragePoolVolumeCopyIntent(spec, poolStatuses[spec.TypedSpec().Pool]),
	}

	resolveErr := ctrl.checkCopyJob(ctx, r, observation)
	if resolveErr == nil {
		resolveErr = ctrl.recordCopyBinding(ctx, r, spec, poolStatuses)
	}

	if resolveErr == nil {
		resolveErr = ctrl.checkCopyJob(ctx, r, observation)
	}

	return resolveErr
}

// recordCopyBinding serializes publication against current disk intent. A withdrawn A
// request must never overwrite B's completed binding just because A's copy finished late.
func cleanupCopyBindingFences(ctx context.Context, r controller.ReaderWriter) error {
	intents, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, r)
	if err != nil {
		return err
	}

	for intent := range intents.All() {
		if !intent.Metadata().Finalizers().Has(libvirtstorage.CopyBindingPublicationFinalizer) {
			continue
		}

		wanted, err := copyBindingFenceWanted(ctx, r, intent)
		if err != nil {
			return err
		}

		if !wanted {
			if err := r.RemoveFinalizer(ctx, intent.Metadata(), libvirtstorage.CopyBindingPublicationFinalizer); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyBindingFenceWanted(ctx context.Context, r controller.Reader, intent *hypervisor.VirtualMachineDiskStatus) (bool, error) {
	if intent.Metadata().Phase() != resource.PhaseRunning {
		return false, nil
	}

	spec, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, storage.StoragePoolVolumeID(intent.TypedSpec().Pool, intent.TypedSpec().Volume))
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	key, _ := intent.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	specKey, _ := spec.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)

	return spec.Metadata().Owner() == "hypervisor.VirtualMachineDiskController" &&
		spec.Metadata().Phase() == resource.PhaseRunning && spec.TypedSpec().Copy != nil && key != "" && key == specKey, nil
}

func sameCopyRequest(current, wanted *storage.StoragePoolVolumeSpec) bool {
	return current.Metadata().Phase() == resource.PhaseRunning &&
		current.Metadata().Created().Equal(wanted.Metadata().Created()) &&
		reflect.DeepEqual(current.TypedSpec(), wanted.TypedSpec())
}

func (ctrl *StoragePoolVolumeController) recordCopyBinding(
	ctx context.Context, r controller.ReaderWriter, spec *storage.StoragePoolVolumeSpec,
	pools map[string]*storage.StoragePoolStatus,
) error {
	key, ok := spec.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	if !ok {
		return nil
	}

	pool := pools[spec.TypedSpec().Pool]
	if pool == nil || pool.TypedSpec().TargetPath == "" {
		return pending("copy binding has no observed pool backing")
	}

	current, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, spec.Metadata().ID())
	if err != nil {
		return err
	}

	if !sameCopyRequest(current, spec) {
		return pending("copy request is withdrawing")
	}

	intent, err := copyBindingIntent(ctx, r, key, spec.TypedSpec())
	if err != nil {
		return err
	}

	if intent == nil {
		return pending("copy is awaiting current disk binding intent")
	}

	// Re-qualifying an unchanged binding does not mutate its name and emits no fence events.
	name, readErr := copyowner.ReadBinding(pool.TypedSpec().TargetPath, key)
	if readErr == nil && name == spec.TypedSpec().Name {
		return ctrl.requalifyCopyBinding(ctx, r, intent, pool.TypedSpec().TargetPath, key, name)
	}

	return ctrl.publishCopyBinding(ctx, r, intent, spec, pool.TypedSpec().TargetPath, key)
}

func (ctrl *StoragePoolVolumeController) requalifyCopyBinding(
	ctx context.Context,
	r controller.ReaderWriter,
	intent *hypervisor.VirtualMachineDiskStatus,
	target, key, name string,
) error {
	if err := ctrl.bindings.Record(target, key, name); err != nil {
		return err
	}
	// A previous publication may have synced successfully but failed to release
	// its fence. Reestablish durability before retrying that acknowledgement.
	if intent.Metadata().Finalizers().Has(libvirtstorage.CopyBindingPublicationFinalizer) {
		return r.RemoveFinalizer(ctx, intent.Metadata(), libvirtstorage.CopyBindingPublicationFinalizer)
	}

	return nil
}

func (ctrl *StoragePoolVolumeController) publishCopyBinding(
	ctx context.Context,
	r controller.ReaderWriter,
	intent *hypervisor.VirtualMachineDiskStatus,
	spec *storage.StoragePoolVolumeSpec,
	target, key string,
) (resultErr error) {
	if err := r.AddFinalizer(ctx, intent.Metadata(), libvirtstorage.CopyBindingPublicationFinalizer); err != nil {
		return err
	}
	defer func() {
		err := r.RemoveFinalizer(context.Background(), intent.Metadata(), libvirtstorage.CopyBindingPublicationFinalizer)
		if !state.IsNotFoundError(err) {
			resultErr = errors.Join(resultErr, err)
		}
	}()

	current, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, spec.Metadata().ID())
	if err != nil {
		return err
	}

	if !sameCopyRequest(current, spec) {
		return pending("copy request is withdrawing")
	}
	// The finalizer changes the intent's version. Re-read after acquisition, and keep
	// it held through the filesystem publication and durability acknowledgement.
	wanted, err := copyBindingIntent(ctx, r, key, spec.TypedSpec())
	if err != nil {
		return err
	}

	if wanted == nil || wanted.Metadata().ID() != intent.Metadata().ID() {
		return pending("copy binding intent changed during publication")
	}

	return ctrl.bindings.Record(target, key, spec.TypedSpec().Name)
}

func copyBindingIntent(ctx context.Context, r controller.Reader, key string, spec *storage.StoragePoolVolumeSpecSpec) (*hypervisor.VirtualMachineDiskStatus, error) {
	statuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, r)
	if err != nil {
		return nil, err
	}

	for status := range statuses.All() {
		binding, _ := status.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
		if status.Metadata().Owner() == "hypervisor.VirtualMachineDiskController" &&
			status.Metadata().Phase() == resource.PhaseRunning && binding == key &&
			status.TypedSpec().Pool == spec.Pool && status.TypedSpec().Volume == spec.Name {
			return status, nil
		}
	}

	return nil, nil
}

// volumeMutation records the outcome only for a marker claimed by this pass.
type volumeMutation struct {
	release func() error
	outcome libvirtstorage.OperationOutcome
}

// publishVolumeResolution owns the marker retirement boundary. The first error
// is a post-publication release failure; the second is a fatal publication error.
func publishVolumeResolution(
	ctx context.Context, r controller.ReaderWriter, spec *storage.StoragePoolVolumeSpec,
	resolved storage.StoragePoolVolumeStatusSpec, resolveErr error, mutation *volumeMutation,
	logger *zap.Logger,
) (error, error) {
	if err := publishVolumeStatus(ctx, r, spec, resolved, resolveErr, logger); err != nil {
		if mutation.release != nil && mutationevidence.CanRelease(mutation.outcome, false) {
			if releaseErr := mutation.release(); releaseErr != nil {
				err = errors.Join(err, fmt.Errorf("release volume mutation marker: %w", releaseErr))
			}
		}

		return nil, err
	}

	// Finished may have partially changed backing, so retire its marker only
	// after replacement status is published. Unknown always remains fenced.
	if mutation.release != nil && mutationevidence.CanRelease(mutation.outcome, true) {
		return mutation.release(), nil
	}

	return nil, nil
}

// volumeStatusWithdrawing closes consumer admission before changing backing identity.
// Teardown preserves the old observation until every attached consumer releases it.
func volumeStatusWithdrawing(ctx context.Context, r controller.ReaderWriter, spec *storage.StoragePoolVolumeSpec) (bool, error) {
	status, err := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, r, spec.Metadata().ID())
	if state.IsNotFoundError(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("error reading volume status %q: %w", spec.Metadata().ID(), err)
	}

	if status.Metadata().Phase() != resource.PhaseRunning {
		return true, nil
	}

	if status.TypedSpec().Pool == spec.TypedSpec().Pool && status.TypedSpec().Name == spec.TypedSpec().Name {
		return false, nil
	}

	if _, err = r.Teardown(ctx, status.Metadata()); err != nil {
		return false, fmt.Errorf("error withdrawing retargeted volume status %q: %w", spec.Metadata().ID(), err)
	}

	return true, nil
}

func publishVolumeStatus(
	ctx context.Context,
	r controller.ReaderWriter,
	spec *storage.StoragePoolVolumeSpec,
	resolved storage.StoragePoolVolumeStatusSpec,
	resolveErr error,
	logger *zap.Logger,
) error {
	var succeeded bool

	err := safe.WriterModify(ctx, r,
		storage.NewStoragePoolVolumeStatus(storage.NamespaceName, spec.Metadata().ID()),
		func(status *storage.StoragePoolVolumeStatus) error {
			succeeded = copySucceededTransition(status.TypedSpec().Copy, resolved.Copy)

			if resolved.Phase == storage.StoragePoolVolumePhaseObservationUnavailable {
				resolved.Path = status.TypedSpec().Path
				resolved.Format = status.TypedSpec().Format
				resolved.Capacity = status.TypedSpec().Capacity
			}

			*status.TypedSpec() = resolved
			status.TypedSpec().Pool = spec.TypedSpec().Pool
			status.TypedSpec().Name = spec.TypedSpec().Name

			if resolveErr != nil {
				status.TypedSpec().Error = resolveErr.Error()
			}

			return nil
		},
	)
	if err == nil && succeeded {
		logger.Debug("copy provisioning succeeded",
			zap.String("storage_pool", spec.TypedSpec().Pool),
			zap.String("volume", spec.TypedSpec().Name),
			zap.String("copy_intent", resolved.Copy.Intent),
		)
	}

	return err
}

func copySucceededTransition(previous, next *storage.StoragePoolVolumeCopyStatusSpec) bool {
	// A growth operation may temporarily publish Waiting for the same initialized
	// disk. Only an actual copy worker or a new initialization merits this log.
	return next != nil && next.Phase == storage.StoragePoolVolumeCopyPhaseSucceeded &&
		(previous == nil || previous.Intent != next.Intent || previous.Phase == storage.StoragePoolVolumeCopyPhaseRunning)
}

// holdSpec takes a hold on the spec, so the volume's existence is not withdrawn before its status is.
func (ctrl *StoragePoolVolumeController) holdSpec(ctx context.Context, r controller.ReaderWriter, spec *storage.StoragePoolVolumeSpec) error {
	if spec.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	if err := r.AddFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("error holding storage pool volume spec %q: %w", spec.Metadata().ID(), err)
	}

	return nil
}

// releaseSpecs gives back the hold on every spec whose status is gone.
func (ctrl *StoragePoolVolumeController) releaseSpecs(
	ctx context.Context, r controller.ReaderWriter, specs safe.List[*storage.StoragePoolVolumeSpec], wanted map[resource.ID]struct{},
) error {
	for spec := range specs.All() {
		id := spec.Metadata().ID()

		if _, keep := wanted[id]; keep || !spec.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if job := ctrl.copies.jobs[id]; job != nil {
			continue
		}

		// The status is withdrawn first: releasing the spec while a status still names the volume
		// would leave nothing to report it through.
		switch _, err := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, r, id); {
		case err == nil:
			continue
		case !state.IsNotFoundError(err):
			return fmt.Errorf("error getting storage pool volume status %q: %w", id, err)
		}

		if err := r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error releasing storage pool volume spec %q: %w", id, err)
		}
	}

	return nil
}

// holdPool keeps a pool's directory in place for as long as a volume of it is asked for. One which
// is tearing down is refused rather than held, as holding it now would block that teardown forever.
func (ctrl *StoragePoolVolumeController) holdPool(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, pool *storage.StoragePoolStatus,
) error {
	if pool.Metadata().Phase() != resource.PhaseRunning {
		return pending("storage pool %q is going away", pool.Metadata().ID())
	}

	if pool.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	if err := r.AddFinalizer(ctx, pool.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("error holding storage pool %q: %w", pool.Metadata().ID(), err)
	}

	logger.Info("holding storage pool for a volume", zap.String("storage_pool", pool.Metadata().ID()))

	return nil
}

// releasePools gives back the hold on every pool nothing needs any more.
//
// held names pools resolved this pass plus pools named by consumer-held volume statuses.
func (ctrl *StoragePoolVolumeController) releasePools(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	poolStatuses map[string]*storage.StoragePoolStatus, held map[string]struct{},
) error {
	// Consumers validate phase after acquiring a hold. Refresh after output teardown
	// so attachments admitted before withdrawal retain their backing pool.
	_, consumers, err := volumesInUse(ctx, r)
	if err != nil {
		return err
	}

	for id := range consumers {
		held[id] = struct{}{}
	}

	for id, pool := range poolStatuses {
		if _, used := held[id]; used || !pool.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, pool.Metadata(), ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error releasing storage pool %q: %w", id, err)
		}

		logger.Info("released storage pool held for a volume", zap.String("storage_pool", id))
	}

	return nil
}

// volumesInUse names consumer-held volumes and their pools, independently of consumer type.
func volumesInUse(ctx context.Context, r controller.Reader) (volumes, pools map[string]struct{}, err error) {
	statuses, err := safe.ReaderListAll[*storage.StoragePoolVolumeStatus](ctx, r)
	if err != nil {
		return nil, nil, fmt.Errorf("error listing storage pool volume statuses: %w", err)
	}

	volumes = map[string]struct{}{}
	pools = map[string]struct{}{}

	for status := range statuses.All() {
		if status.Metadata().Finalizers().Empty() {
			continue
		}

		spec := status.TypedSpec()

		if spec.Pool != "" && spec.Name != "" {
			volumes[storage.StoragePoolVolumeID(spec.Pool, spec.Name)] = struct{}{}
			pools[spec.Pool] = struct{}{}
		}
	}

	return volumes, pools, nil
}

// resolve makes one volume be what the spec asks for, as far as it safely can.
//
// What it will not do is as much the point as what it will: it never deletes, never shrinks, never
// rewrites a volume whose format differs from the one asked for, and never resizes one a guest has
// open. Each of those is reported rather than forced.
func (ctrl *StoragePoolVolumeController) resolve(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	spec *storage.StoragePoolVolumeSpecSpec,
	poolStatuses map[string]*storage.StoragePoolStatus,
	held map[string]struct{},
	openSession func() (libvirtstorage.VolumeClient, error),
	mutation *volumeMutation,
) (storage.StoragePoolVolumeStatusSpec, error) {
	// A canceled attempt retains its slot and dependencies until cleanup, even
	// when its replacement is invalid or now asks for a blank volume.
	if job := ctrl.copies.jobs[storage.StoragePoolVolumeID(spec.Pool, spec.Name)]; job != nil {
		status := copyObservation(job.phase())
		if spec.Copy == nil {
			status.Copy = nil
		}

		return status, pending("copy volume %q is in progress", spec.Name)
	}

	pool, err := ctrl.resolvePool(ctx, r, logger, spec, poolStatuses, held)
	if err != nil {
		status := storage.StoragePoolVolumeStatusSpec{
			Phase: storage.StoragePoolVolumePhaseNotReady,
		}
		if errors.Is(err, libvirtstorage.ErrVolumeObservationUnavailable) {
			status.Phase = storage.StoragePoolVolumePhaseObservationUnavailable
		}

		return status, err
	}

	// A previous resize may still be running in the daemon after its transport
	// returned an error. A later lookup, even at the requested size, cannot
	// establish that the operation is complete.
	currentPool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, spec.Pool)
	if err != nil {
		return storage.StoragePoolVolumeStatusSpec{Phase: storage.StoragePoolVolumePhaseNotReady}, err
	}

	if currentPool.Metadata().Finalizers().Has(storage.StoragePoolVolumeMutationFinalizer(spec.Name)) {
		return storage.StoragePoolVolumeStatusSpec{Phase: storage.StoragePoolVolumePhaseNotReady},
			pending("volume %q in storage pool %q has an uncertain prior resize; recovery requires confirming the daemon is quiescent before retiring its mutation marker", spec.Name, spec.Pool)
	}

	session, err := openSession()
	if err != nil {
		// Failure to open the daemon is lost observation, even when the pool
		// controller has not yet observed the same outage.
		return storage.StoragePoolVolumeStatusSpec{
			Phase: storage.StoragePoolVolumePhaseObservationUnavailable,
		}, err
	}

	return ctrl.resolveVolume(ctx, r, session, logger, pool, poolStatuses[spec.Pool], spec, mutation)
}

func (ctrl *StoragePoolVolumeController) resolveVolume(
	ctx context.Context, r controller.ReaderWriter, session libvirtstorage.VolumeClient, logger *zap.Logger,
	pool libvirtstorage.Pool, poolStatus *storage.StoragePoolStatus, spec *storage.StoragePoolVolumeSpecSpec,
	mutation *volumeMutation,
) (storage.StoragePoolVolumeStatusSpec, error) {
	if spec.Copy != nil {
		return ctrl.copyResolution(ctx, r, session, logger, pool, poolStatus, spec, mutation)
	}

	volume, exists, err := session.Volume(pool, spec.Name)
	if err != nil {
		phase := storage.StoragePoolVolumePhaseNotReady
		if errors.Is(err, libvirtstorage.ErrVolumeObservationUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			phase = storage.StoragePoolVolumePhaseObservationUnavailable
		}

		return storage.StoragePoolVolumeStatusSpec{Phase: phase}, err
	}

	if !exists {
		if volume, err = session.CreateVolume(pool, spec.Name, spec.Format, spec.Capacity, spec.BackingFile, spec.BackingFormat); err != nil {
			return storage.StoragePoolVolumeStatusSpec{
				Phase: storage.StoragePoolVolumePhaseNotReady,
			}, err
		}

		logger.Info("created storage pool volume",
			zap.String("storage_pool", spec.Pool), zap.String("volume", spec.Name),
			zap.String("format", spec.Format), zap.Uint64("capacity", spec.Capacity))

		return readyVolume(volume), nil
	}

	return ctrl.adopt(ctx, r, session, logger, pool, poolStatus, spec, volume, mutation)
}

func copyVolumeFailure(err error) (storage.StoragePoolVolumeStatusSpec, error) {
	phase := storage.StoragePoolVolumePhaseNotReady

	if errors.Is(err, libvirtstorage.ErrCopySourceRequired) {
		return copyObservation(storage.StoragePoolVolumeCopyPhaseSourceRequired), pending("%w", err)
	}

	if errors.Is(err, libvirtstorage.ErrVolumeObservationUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		phase = storage.StoragePoolVolumePhaseObservationUnavailable
	}

	return storage.StoragePoolVolumeStatusSpec{Phase: phase}, err
}

// resolvePool holds the pool a volume lives in and names it to libvirt, or says why it cannot yet.
func (ctrl *StoragePoolVolumeController) resolvePool(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	spec *storage.StoragePoolVolumeSpecSpec,
	poolStatuses map[string]*storage.StoragePoolStatus,
	held map[string]struct{},
) (libvirtstorage.Pool, error) {
	name := spec.Pool

	poolStatus, found := poolStatuses[name]
	if !found {
		return libvirtstorage.Pool{}, pending(
			"storage pool %q has no status yet: it is either not declared by a StoragePoolConfig document, or not reconciled",
			name)
	}

	if poolStatus.TypedSpec().Phase != storage.StoragePoolPhaseReady {
		if poolStatus.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable {
			if err := recheckHeldPool(ctx, r, name, spec.Name, poolStatus); err != nil {
				return libvirtstorage.Pool{}, err
			}
		}

		// A pending pool cannot be used for new volume work. Let an idle volume give
		// its pool hold back so a retarget can complete; consumer-held statuses
		// still pin the backing pool.
		return libvirtstorage.Pool{}, pending("storage pool %q is not ready: %s", name, poolStatus.TypedSpec().Error)
	}

	if err := ctrl.holdPool(ctx, r, logger, poolStatus); err != nil {
		return libvirtstorage.Pool{}, err
	}

	// The snapshot above predates our hold. Re-read after committing the hold:
	// a pool retarget may have published exclusion in the meantime.
	if err := recheckHeldPool(ctx, r, name, spec.Name, poolStatus); err != nil {
		return libvirtstorage.Pool{}, err
	}

	held[name] = struct{}{}

	machineUUID, err := poolMachineUUID(ctx, r)
	if err != nil {
		// Reported rather than retried: at boot the machine's own identity may simply not have been
		// read yet, and the SystemInformation input is what says when it has.
		return libvirtstorage.Pool{}, pending("%w", err)
	}

	return libvirtstorage.Pool{Name: name, UUID: libvirtstorage.UUID(machineUUID, name)}, nil
}

//nolint:gocyclo // Check lifecycle, backing identity and exclusion before classifying observation loss.
func recheckHeldPool(ctx context.Context, r controller.Reader, name, volume string, poolStatus *storage.StoragePoolStatus, ownMarkers ...string) error {
	current, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, name)
	if err != nil {
		return fmt.Errorf("error rechecking storage pool %q: %w", name, err)
	}

	if current.Metadata().Phase() != resource.PhaseRunning || !current.Metadata().Created().Equal(poolStatus.Metadata().Created()) ||
		current.TypedSpec().VolumeID != poolStatus.TypedSpec().VolumeID || current.TypedSpec().TargetPath != poolStatus.TypedSpec().TargetPath {
		return pending("storage pool %q changed while acquiring its hold", name)
	}

	for _, finalizer := range *current.Metadata().Finalizers() {
		if slices.Contains(ownMarkers, finalizer) {
			continue
		}

		if strings.HasPrefix(finalizer, "storage.StoragePoolController/mutating/") {
			return pending("storage pool %q is changing backing", name)
		}

		if volume == "" && strings.HasPrefix(finalizer, storage.StoragePoolVolumeMutationFinalizerPrefix) {
			return pending("storage pool %q has an uncertain volume mutation; private cleanup waits for its retirement", name)
		}

		if finalizer == storage.StoragePoolVolumeMutationFinalizer(volume) {
			return pending("volume %q in storage pool %q has an uncertain prior resize; recovery requires confirming the daemon is quiescent before retiring its mutation marker", volume, name)
		}
	}

	if current.TypedSpec().Phase != storage.StoragePoolPhaseReady && current.TypedSpec().Phase != storage.StoragePoolPhaseObservationUnavailable {
		return pending("storage pool %q changed while acquiring its hold", name)
	}

	// Only loss of observation on the same running resource and backing can
	// preserve a guest. Withdrawal, recreation and mutation exclusion win above.
	// If observation recovered since the snapshot, defer work until its event
	// without briefly reporting a backing failure to an already running guest.
	if current.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable || poolStatus.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable {
		return pending("storage pool %q: %w", name, libvirtstorage.ErrVolumeObservationUnavailable)
	}

	return nil
}

// adopt takes a volume already in the pool as the one the spec means, growing it if it may.
//
// That is not a fallback, it is the ordinary case: after a reboot the volume is always already
// there, and a configuration naming the same virtual machine, disk and pool is saying it means the
// same disk.
func (ctrl *StoragePoolVolumeController) adopt(
	ctx context.Context,
	r controller.ReaderWriter,
	session libvirtstorage.VolumeClient,
	logger *zap.Logger,
	pool libvirtstorage.Pool,
	poolStatus *storage.StoragePoolStatus,
	spec *storage.StoragePoolVolumeSpecSpec,
	volume libvirtstorage.Volume,
	mutation *volumeMutation,
) (storage.StoragePoolVolumeStatusSpec, error) {
	if volume.Format != spec.Format {
		// Never rewritten: attaching a qcow2 file as raw hands the guest its header as block zero,
		// and the guest writes over it.
		return storage.StoragePoolVolumeStatusSpec{
				Phase:    storage.StoragePoolVolumePhaseNotReady,
				Capacity: volume.Capacity,
				Format:   volume.Format,
				Path:     volume.Path,
			},
			pending("volume %q in storage pool %q is %s, but %s was asked for: change the format back, or move the file aside",
				spec.Name, spec.Pool, volume.Format, spec.Format)
	}

	if volume.Capacity >= spec.Capacity {
		status := readyVolume(volume)

		if volume.Capacity > spec.Capacity {
			// Reported, not applied, and not a reason to refuse the disk: a volume larger than
			// asked for is perfectly attachable, and stopping a running guest over an edited number
			// is a worse outcome than the mismatch.
			return status, pending("volume %q in storage pool %q is %d bytes, larger than the %d asked for: a volume is never shrunk",
				spec.Name, spec.Pool, volume.Capacity, spec.Capacity)
		}

		return status, nil
	}

	// A held volume needs no mutation at all. This first census is only a fast
	// deferral path, never permission to resize: avoid publishing markers on
	// every pass for a running guest, which would wake this controller forever.
	inUse, _, err := volumesInUse(ctx, r)
	if err != nil {
		return storage.StoragePoolVolumeStatusSpec{
			Phase: storage.StoragePoolVolumePhaseNotReady,
		}, err
	}

	if ctrl.volumeIsOpen(spec, inUse) {
		return deferredGrowth(spec, volume)
	}

	return ctrl.growVolume(ctx, r, session, logger, pool, poolStatus, spec, volume, mutation)
}

// growVolume excludes starts while a fresh hold census admits a destructive resize.
func (ctrl *StoragePoolVolumeController) growVolume(
	ctx context.Context,
	r controller.ReaderWriter,
	session libvirtstorage.VolumeClient,
	logger *zap.Logger,
	pool libvirtstorage.Pool,
	poolStatus *storage.StoragePoolStatus,
	spec *storage.StoragePoolVolumeSpecSpec,
	volume libvirtstorage.Volume,
	mutation *volumeMutation,
) (storage.StoragePoolVolumeStatusSpec, error) {
	marker := storage.StoragePoolVolumeMutationFinalizer(spec.Name)
	if err := claimVolumeResize(ctx, r, poolStatus, spec, marker); err != nil {
		return storage.StoragePoolVolumeStatusSpec{Phase: storage.StoragePoolVolumePhaseNotReady}, err
	}

	// This invocation owns a pre-submission marker. The caller decides whether
	// publication is required before retiring it.
	mutation.outcome = libvirtstorage.NoMutationSubmitted
	mutation.release = func() error {
		err := r.RemoveFinalizer(ctx, poolStatus.Metadata(), marker)
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	inUse, _, err := volumesInUse(ctx, r)
	if err != nil {
		return storage.StoragePoolVolumeStatusSpec{
			Phase: storage.StoragePoolVolumePhaseNotReady,
		}, err
	}

	if ctrl.volumeIsOpen(spec, inUse) {
		return deferredGrowth(spec, volume)
	}

	if err := recheckHeldPool(ctx, r, spec.Pool, spec.Name, poolStatus, marker); err != nil {
		return storage.StoragePoolVolumeStatusSpec{Phase: storage.StoragePoolVolumePhaseNotReady}, err
	}

	if err := resizeVolumeWithEvidence(session, pool, spec, mutation); err != nil {
		return storage.StoragePoolVolumeStatusSpec{Phase: storage.StoragePoolVolumePhaseNotReady}, err
	}

	logger.Info("grew storage pool volume",
		zap.String("storage_pool", spec.Pool), zap.String("volume", spec.Name),
		zap.Uint64("from", volume.Capacity), zap.Uint64("to", spec.Capacity))

	// Not looked up again: the resize passed no flags, so libvirt set the capacity outright, and
	// neither the path nor the format is a thing a resize changes.
	volume.Capacity = spec.Capacity

	return readyVolume(volume), nil
}

// resizeVolumeWithEvidence keeps exclusion when the daemon may still be resizing.
func resizeVolumeWithEvidence(
	session libvirtstorage.VolumeClient, pool libvirtstorage.Pool, spec *storage.StoragePoolVolumeSpecSpec, mutation *volumeMutation,
) error {
	outcome, err := session.ResizeVolumeOperation(pool, spec.Name, spec.Capacity)
	mutation.outcome = mutationevidence.Merge(mutation.outcome, outcome)

	if err != nil {
		return err
	}

	if mutation.outcome != libvirtstorage.Finished {
		return errResizeUncertain
	}

	return nil
}

// claimVolumeResize distinguishes an inherited marker from one acquired this
// pass, then publishes exclusion before the fresh consumer census.
func claimVolumeResize(
	ctx context.Context, r controller.ReaderWriter, poolStatus *storage.StoragePoolStatus,
	spec *storage.StoragePoolVolumeSpecSpec, marker string,
) error {
	current, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, spec.Pool)
	if err != nil {
		return err
	}

	if current.Metadata().Finalizers().Has(marker) {
		return pending("volume %q in storage pool %q has an uncertain prior resize; recovery requires confirming the daemon is quiescent before retiring its mutation marker", spec.Name, spec.Pool)
	}

	// A VM takes its volume holds BEFORE reading this marker: whichever
	// writes second observes the other.
	return r.AddFinalizer(ctx, poolStatus.Metadata(), marker)
}

// deferredGrowth keeps a held disk attachable without mutating a running guest's volume.
func deferredGrowth(spec *storage.StoragePoolVolumeSpecSpec, volume libvirtstorage.Volume) (storage.StoragePoolVolumeStatusSpec, error) {
	status := readyVolume(volume)
	status.PendingCapacity = spec.Capacity

	return status, pending("volume %q in storage pool %q cannot be grown to %d bytes while a consumer has it attached: detach all consumers to apply growth",
		spec.Name, spec.Pool, spec.Capacity)
}

// volumeIsOpen reports whether an attached consumer holds this volume's status.
func (ctrl *StoragePoolVolumeController) volumeIsOpen(spec *storage.StoragePoolVolumeSpecSpec, inUse map[string]struct{}) bool {
	_, open := inUse[storage.StoragePoolVolumeID(spec.Pool, spec.Name)]

	return open
}

// readyVolume reports a volume which may be attached.
func readyVolume(volume libvirtstorage.Volume) storage.StoragePoolVolumeStatusSpec {
	return storage.StoragePoolVolumeStatusSpec{
		Path:     volume.Path,
		Format:   volume.Format,
		Capacity: volume.Capacity,
		Phase:    storage.StoragePoolVolumePhaseReady,
	}
}

type copyJob struct {
	id         resource.ID
	spec       storage.StoragePoolVolumeSpecSpec
	pool       libvirtstorage.Pool
	bindingKey string

	reader controller.Reader
	ctx    context.Context //nolint:containedctx // The admitted job owns this cancellable lifetime until its worker acknowledges cleanup.
	cancel context.CancelFunc

	intent   string
	err      error
	terminal bool
}

// copyProof keeps adoption and attempt evidence qualified by semantic intent and backing.
// It is not an admission lock: only jobs owns worker lifetimes.
type copyProof struct {
	intent   string
	cache    libvirtstorage.CopyCache
	err      error
	retryAt  time.Time
	failures uint8
	terminal bool
}

const copyRetryMax = time.Minute

// deferredCopyRetryError keeps the diagnostic and tells the Run timer when the next
// attempt is eligible; ordinary volume failures still get the one-second retry.
type deferredCopyRetryError struct {
	error
	at time.Time
}

func (e deferredCopyRetryError) Unwrap() error {
	return e.error
}

func volumeRetryDelay(errs []error, now time.Time) time.Duration {
	var delay time.Duration

	for _, err := range errs {
		candidate := time.Second

		if deferred, ok := errors.AsType[deferredCopyRetryError](err); ok {
			candidate = deferred.at.Sub(now)
			if candidate <= 0 {
				candidate = time.Millisecond
			}
		}

		if delay == 0 || candidate < delay {
			delay = candidate
		}
	}

	return delay
}

type copyCursor struct {
	pool     string
	requests map[string]resource.ID
}

func (cursor *copyCursor) prune(pools map[string]*storage.StoragePoolStatus) {
	for name := range cursor.requests {
		if pools[name] == nil {
			delete(cursor.requests, name)
		}
	}
}

func compareCopyCursor(a, b, cursor string) int {
	if (a > cursor) != (b > cursor) {
		if a > cursor {
			return -1
		}

		return 1
	}

	return cmp.Compare(a, b)
}

type copyWork struct {
	logger   *zap.Logger
	ctx      context.Context //nolint:containedctx // Owned and joined by the controller Run.
	cancel   context.CancelFunc
	finished chan *copyJob
	jobs     map[resource.ID]*copyJob
	proofs   map[resource.ID]*copyProof
	cursor   copyCursor
	wg       sync.WaitGroup
}

func (ctrl *StoragePoolVolumeController) startCopies(ctx context.Context) *copyWork {
	if ctrl.Open == nil {
		ctrl.Open = func(ctx context.Context) (libvirtstorage.VolumeClient, error) {
			return libvirt.New().StorageVolumes(ctx)
		}
	}

	workCtx, cancel := context.WithCancel(ctx)

	return &copyWork{
		ctx:      workCtx,
		cancel:   cancel,
		finished: make(chan *copyJob, copyMaxRunning),
		jobs:     make(map[resource.ID]*copyJob),
		proofs:   make(map[resource.ID]*copyProof),
		cursor:   copyCursor{requests: make(map[string]resource.ID)},
	}
}

// complete runs only after the operation and both short sessions have closed.
func (w *copyWork) complete(job *copyJob) {
	if w.jobs[job.id] != job {
		return
	}

	delete(w.jobs, job.id)
	job.cancel()

	if proof := w.proofs[job.id]; proof != nil && proof.intent == job.intent {
		proof.recordAttempt(job.err, job.terminal, time.Now())
	}
}

func (proof *copyProof) recordAttempt(err error, terminal bool, now time.Time) {
	proof.err, proof.terminal = err, terminal
	if err == nil || terminal {
		proof.failures = 0
		proof.retryAt = time.Time{}

		return
	}

	delay := time.Second
	for n := uint8(0); n < proof.failures && delay < copyRetryMax; n++ {
		delay = min(delay*2, copyRetryMax)
	}

	if proof.failures < 7 {
		proof.failures++
	}

	proof.retryAt = now.Add(delay)
}

func (w *copyWork) stop() {
	w.cancel()

	for _, job := range w.jobs {
		job.cancel()
	}

	w.wg.Wait()
}

func copyProgressObserver(logger *zap.Logger) func(libvirtstorage.CopyProgress) {
	if logger == nil || !logger.Core().Enabled(zap.DebugLevel) {
		return nil
	}

	var (
		last   libvirtstorage.CopyProgress
		logged bool
	)

	return func(progress libvirtstorage.CopyProgress) {
		if logged && progress.Phase == last.Phase && progress.Elapsed-last.Elapsed < 5*time.Second {
			return
		}

		fields := []zap.Field{
			zap.String("copy_phase", string(progress.Phase)),
			zap.Uint64("copied_bytes", progress.BytesCopied),
			zap.Duration("elapsed", progress.Elapsed),
		}
		if progress.TotalKnown {
			fields = append(fields, zap.Uint64("total_bytes", progress.TotalBytes))
		}

		if elapsed := progress.Elapsed - last.Elapsed; logged && elapsed > 0 && progress.BytesCopied >= last.BytesCopied {
			fields = append(fields, zap.Float64("bytes_per_second", float64(progress.BytesCopied-last.BytesCopied)/elapsed.Seconds()))
		}

		logger.Debug("copy provisioning progress", fields...)

		last = progress
		logged = true
	}
}

func (ctrl *StoragePoolVolumeController) executeCopy(job *copyJob) (libvirtstorage.Volume, error) {
	logger := ctrl.copies.logger
	if logger == nil || !logger.Core().Enabled(zap.DebugLevel) {
		return ctrl.transferCopy(job, nil)
	}

	logger = logger.With(
		zap.String("storage_pool", job.spec.Pool),
		zap.String("volume", job.spec.Name),
		zap.String("copy_intent", job.intent),
	)
	started := time.Now()

	logger.Debug("copy worker started")

	volume, err := ctrl.transferCopy(job, copyProgressObserver(logger))
	outcome := "transferred"

	switch {
	case job.ctx.Err() != nil:
		outcome = "canceled"
	case err != nil:
		outcome = "failed"
	}

	// transferCopy has closed the private stage, destination root and both
	// short sessions. Transfer completion is not attachment readiness.
	logger.Debug("copy worker finished",
		zap.String("outcome", outcome),
		zap.Duration("elapsed", time.Since(started)),
		zap.Error(err),
	)

	return volume, err
}

func (ctrl *StoragePoolVolumeController) transferCopy(job *copyJob, observer func(libvirtstorage.CopyProgress)) (libvirtstorage.Volume, error) {
	preparation, err := ctrl.Open(job.ctx)
	if err != nil {
		return libvirtstorage.Volume{}, err
	}

	op, err := preparation.PrepareCopy(job.pool, job.spec.Name, job.spec.Copy.Source, job.spec.Copy.Digest)
	preparation.Close()

	if err != nil {
		return libvirtstorage.Volume{}, err
	}

	if op != nil {
		defer op.Close()

		op.SetProgressObserver(observer)

		if err := bindConfiguredCopyOwner(job, op); err != nil {
			return libvirtstorage.Volume{}, err
		}

		if err = ctrl.transferCopyStage(job, op); err != nil {
			job.terminal = job.isTerminalTransferError(err)

			return libvirtstorage.Volume{}, err
		}
	}

	if err = ctrl.checkCopyJob(job.ctx, job.reader, job); err != nil {
		return libvirtstorage.Volume{}, err
	}

	validation, err := ctrl.Open(job.ctx)
	if err != nil {
		return libvirtstorage.Volume{}, err
	}
	defer validation.Close()

	volume, err := validation.FinishCopy(job.ctx, job.pool, op, func() error {
		return ctrl.checkCopyJob(job.ctx, job.reader, job)
	})
	job.terminal = (errors.Is(err, libvirtstorage.ErrCopyFormatUnsupported) || qemuimg.IsTerminal(err)) && job.ctx.Err() == nil

	return volume, err
}

func (job *copyJob) isTerminalTransferError(err error) bool {
	if job.spec.Copy.Image {
		return qemuimg.IsTerminal(err) && job.ctx.Err() == nil
	}

	var pathErr *os.PathError

	return !errors.As(err, &pathErr) && !errors.Is(err, libvirtstorage.ErrCopySourceChanged) && job.ctx.Err() == nil
}

func (ctrl *StoragePoolVolumeController) transferCopyStage(job *copyJob, op *libvirtstorage.CopyOperation) error {
	if !job.spec.Copy.Image {
		return op.Transfer(job.ctx)
	}

	tool := ctrl.ImageTool
	if tool == nil {
		tool = qemuimg.New("")
	}

	return op.Import(job.ctx, tool, job.spec.Format, job.spec.Capacity)
}

func (job *copyJob) phase() storage.StoragePoolVolumeCopyPhase {
	if job.ctx.Err() != nil {
		return storage.StoragePoolVolumeCopyPhaseCancelling
	}

	return storage.StoragePoolVolumeCopyPhaseRunning
}

func (job *copyJob) matches(spec *storage.StoragePoolVolumeSpec, pool *storage.StoragePoolStatus) bool {
	return spec != nil && spec.Metadata().Phase() == resource.PhaseRunning &&
		pool != nil && pool.Metadata().Phase() == resource.PhaseRunning &&
		pool.TypedSpec().Phase == storage.StoragePoolPhaseReady &&
		storage.StoragePoolVolumeCopyIntent(spec, pool) == job.intent
}

func (proof *copyProof) matches(spec *storage.StoragePoolVolumeSpec, pool *storage.StoragePoolStatus) bool {
	return storage.StoragePoolVolumeCopyIntent(spec, pool) == proof.intent
}

func (w *copyWork) fence(specs safe.List[*storage.StoragePoolVolumeSpec], pools map[string]*storage.StoragePoolStatus) {
	w.cursor.prune(pools)

	live := make(map[resource.ID]*storage.StoragePoolVolumeSpec)
	for spec := range specs.All() {
		live[spec.Metadata().ID()] = spec
	}

	for id, job := range w.jobs {
		if !job.matches(live[id], pools[job.spec.Pool]) {
			job.cancel()
		}
	}

	for id, proof := range w.proofs {
		spec := live[id]
		if spec == nil || spec.Metadata().Phase() != resource.PhaseRunning {
			if w.jobs[id] == nil {
				delete(w.proofs, id)
			}

			continue
		}

		pool := pools[spec.TypedSpec().Pool]
		if !proof.matches(spec, pool) {
			delete(w.proofs, id)
		}
	}
}

func copyObservation(phase storage.StoragePoolVolumeCopyPhase) storage.StoragePoolVolumeStatusSpec {
	return storage.StoragePoolVolumeStatusSpec{
		Phase: storage.StoragePoolVolumePhaseNotReady,
		Copy:  &storage.StoragePoolVolumeCopyStatusSpec{Phase: phase},
	}
}

func (ctrl *StoragePoolVolumeController) copyResolution(ctx context.Context, r controller.ReaderWriter, session libvirtstorage.VolumeClient, logger *zap.Logger,
	pool libvirtstorage.Pool, poolStatus *storage.StoragePoolStatus, spec *storage.StoragePoolVolumeSpecSpec,
	mutation *volumeMutation,
) (storage.StoragePoolVolumeStatusSpec, error) {
	id := storage.StoragePoolVolumeID(spec.Pool, spec.Name)

	if !validCopySpec(spec) {
		return copyObservation(storage.StoragePoolVolumeCopyPhaseFailed), pending("copy volume %q requires no backing chain; "+
			"writable image imports require raw/qcow2 format and a positive capacity; immutable media require raw format and zero capacity", spec.Name)
	}

	current, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, id)
	if err != nil {
		return copyVolumeFailure(err)
	}

	if !reflect.DeepEqual(current.TypedSpec(), spec) {
		return copyObservation(storage.StoragePoolVolumeCopyPhaseWaiting), pending("copy request changed during resolution")
	}

	intent := storage.StoragePoolVolumeCopyIntent(current, poolStatus)

	proof := ctrl.copies.proofFor(id, intent, spec.Copy.Image)
	// Recover publication before interpreting any failed attempt or requiring source.
	// Adoption errors leave the outcome unqualified, never permit source replacement.
	volume, adopted, err := session.AdoptCopy(pool, spec.Name, &proof.cache)
	if err != nil {
		return copyVolumeFailure(err)
	}

	if adopted {
		if spec.Copy.Image {
			return ctrl.adopt(ctx, r, session, logger, pool, poolStatus, spec, volume, mutation)
		}

		return readyVolume(volume), nil
	}

	if attemptErr := proof.attemptError(); attemptErr != nil {
		return copyObservation(storage.StoragePoolVolumeCopyPhaseFailed), attemptErr
	}

	if spec.Copy.Source == "" {
		return copyVolumeFailure(libvirtstorage.ErrCopySourceRequired)
	}

	return ctrl.admitCopy(r, pool, poolStatus, current, spec)
}

func (w *copyWork) proofFor(id resource.ID, intent string, image bool) *copyProof {
	proof := w.proofs[id]
	if proof == nil || proof.intent != intent {
		proof = &copyProof{intent: intent}
		proof.cache.Image = image
		w.proofs[id] = proof
	}

	return proof
}

// attemptError keeps rejected attempts dormant and transient attempts on a deadline.
func (proof *copyProof) attemptError() error {
	return proof.attemptErrorAt(time.Now())
}

func (proof *copyProof) attemptErrorAt(now time.Time) error {
	if proof.err == nil {
		return nil
	}

	if proof.terminal {
		return pending("%w", proof.err)
	}

	if now.Before(proof.retryAt) {
		return deferredCopyRetryError{
			error: proof.err,
			at:    proof.retryAt,
		}
	}

	return nil
}

func validCopySpec(spec *storage.StoragePoolVolumeSpecSpec) bool {
	if spec.BackingFile != "" || spec.BackingFormat != "" {
		return false
	}

	if spec.Copy.Image {
		return (spec.Format == "raw" || spec.Format == "qcow2") && spec.Capacity > 0
	}

	return spec.Format == "raw" && spec.Capacity == 0
}

func (ctrl *StoragePoolVolumeController) admitCopy(r controller.Reader, pool libvirtstorage.Pool, poolStatus *storage.StoragePoolStatus,
	current *storage.StoragePoolVolumeSpec, spec *storage.StoragePoolVolumeSpecSpec,
) (storage.StoragePoolVolumeStatusSpec, error) {
	// The single controller owns a node-wide limit across all pools.
	if len(ctrl.copies.jobs) >= copyMaxRunning {
		return copyObservation(storage.StoragePoolVolumeCopyPhaseQueued), pending("copy volume %q is waiting for capacity", spec.Name)
	}

	jobCtx, cancel := context.WithCancel(ctrl.copies.ctx)
	bindingKey, _ := current.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
	job := &copyJob{
		bindingKey: bindingKey,
		id:         current.Metadata().ID(),
		spec:       *spec,
		pool:       pool,
		intent:     storage.StoragePoolVolumeCopyIntent(current, poolStatus),
		reader:     r,
		ctx:        jobCtx,
		cancel:     cancel,
	}
	ctrl.copies.jobs[job.id] = job
	ctrl.copies.cursor.pool = job.spec.Pool
	ctrl.copies.cursor.requests[job.spec.Pool] = job.id
	ctrl.copies.wg.Go(func() {
		_, job.err = ctrl.executeCopy(job)
		select {
		case ctrl.copies.finished <- job:
		case <-ctrl.copies.ctx.Done():
		}
	})

	return copyObservation(storage.StoragePoolVolumeCopyPhaseRunning), pending("copy volume %q is in progress", spec.Name)
}

func (ctrl *StoragePoolVolumeController) checkCopyJob(ctx context.Context, r controller.Reader, job *copyJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	current, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, job.id)
	if err != nil {
		return err
	}

	pool, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, job.spec.Pool)
	if err != nil {
		return err
	}

	if !job.matches(current, pool) {
		// Losing only observation on the same request/backing is not evidence
		// that completed initialization was revoked. Recheck exclusion before
		// classifying it, so teardown, retargeting and mutation still win.
		if current.Metadata().Phase() == resource.PhaseRunning && storage.StoragePoolVolumeCopyIntent(current, pool) == job.intent {
			if err := recheckHeldPool(ctx, r, job.spec.Pool, job.spec.Name, pool); errors.Is(err, libvirtstorage.ErrVolumeObservationUnavailable) {
				return err
			}
		}

		return fmt.Errorf("copy request or pool backing changed during transfer")
	}

	return nil
}
