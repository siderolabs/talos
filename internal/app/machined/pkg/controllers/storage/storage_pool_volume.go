// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/cleanup"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/libvirt"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const volumeControllerName = "storage.StoragePoolVolumeController"

// errVolumePending marks a condition an input event resolves, rather than restart backoff.
var errVolumePending = errors.New("volume pending")

var errResizeUncertain = errors.New("resize outcome uncertain")

// pendingError carries errVolumePending without saying so.
//
// The condition is reported through the volume's status, which an operator reads: prefixing it with
// the sentinel's own text would say "volume pending" of conditions which are not pending at all,
// such as a volume whose format differs from the one asked for.
type pendingError struct{ err error }

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
	Open func(context.Context) (libvirtstorage.Client, error)
}

// Name implements controller.Controller interface.
func (ctrl *StoragePoolVolumeController) Name() string {
	return volumeControllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *StoragePoolVolumeController) Inputs() []controller.Input {
	return []controller.Input{
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
func (ctrl *StoragePoolVolumeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	if ctrl.Open == nil {
		ctrl.Open = func(ctx context.Context) (libvirtstorage.Client, error) {
			return libvirt.New().StorageVolumes(ctx)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		specs, err := safe.ReaderListAll[*storage.StoragePoolVolumeSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing storage pool volume specs: %w", err)
		}

		if err = ctrl.reconcile(ctx, r, logger, specs); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

// reconcile brings every asked-for volume into being, opening a session only if one is needed.
func (ctrl *StoragePoolVolumeController) reconcile(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, specs safe.List[*storage.StoragePoolVolumeSpec],
) error {
	poolStatuses, err := volumePoolStatuses(ctx, r)
	if err != nil {
		return err
	}

	_, poolsInUse, err := volumesInUse(ctx, r)
	if err != nil {
		return err
	}

	// Seeded with the pools a guest already has a volume of open: those stay held whether or not a
	// spec resolves against them this pass, and in particular while one is tearing down.
	held := maps.Clone(poolsInUse)

	var (
		session libvirtstorage.Client
		errs    []error
	)

	defer func() {
		if session != nil {
			session.Close()
		}
	}()

	// Opened lazily, and at most once: a host with no volumes to reconcile must not dial the daemon
	// at all, or every pass turns a daemon outage into a restart loop for nothing.
	openSession := func() (libvirtstorage.Client, error) {
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
		return errors.Join(append(errs, err)...)
	}

	// A status nothing asks for any more is torn down rather than destroyed, so whatever holds it
	// has a chance to notice and give the hold back. The volume's file is untouched either way.
	if err = cleanup.Outputs[*storage.StoragePoolVolumeStatus](ctx, r, "storage pool volume status", wanted); err != nil {
		return errors.Join(append(errs, err)...)
	}

	if err = ctrl.releaseSpecs(ctx, r, specs, wanted); err != nil {
		return errors.Join(append(errs, err)...)
	}

	return errors.Join(append(errs, ctrl.releasePools(ctx, r, logger, poolStatuses, held))...)
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
	openSession func() (libvirtstorage.Client, error),
) (map[resource.ID]struct{}, []error, error) {
	wanted := map[resource.ID]struct{}{}

	var errs []error

	for spec := range specs.All() {
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
	held map[string]struct{}, openSession func() (libvirtstorage.Client, error),
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

	if resolveErr != nil && !errors.Is(resolveErr, errVolumePending) {
		// A libvirt or filesystem failure emits no resource event: retry via backoff.
		errs = append(errs, fmt.Errorf("volume %q: %w", id, resolveErr))
	}

	releaseErr, publishErr := publishVolumeResolution(ctx, r, spec, resolved, resolveErr, &mutation)
	if publishErr != nil {
		return false, errs, fmt.Errorf("error updating storage pool volume status %q: %w", id, publishErr)
	}

	if releaseErr != nil {
		errs = append(errs, fmt.Errorf("volume %q: release mutation marker: %w", id, releaseErr))
	}

	return false, errs, nil
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
) (error, error) {
	if err := publishVolumeStatus(ctx, r, spec, resolved, resolveErr); err != nil {
		if mutation.release != nil && mutation.outcome == libvirtstorage.NoMutationSubmitted {
			if releaseErr := mutation.release(); releaseErr != nil {
				err = errors.Join(err, fmt.Errorf("release volume mutation marker: %w", releaseErr))
			}
		}

		return nil, err
	}

	// Finished may have partially changed backing, so retire its marker only
	// after replacement status is published. Unknown always remains fenced.
	if mutation.release != nil && mutation.outcome != libvirtstorage.Unknown {
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
) error {
	return safe.WriterModify(ctx, r,
		storage.NewStoragePoolVolumeStatus(storage.NamespaceName, spec.Metadata().ID()),
		func(status *storage.StoragePoolVolumeStatus) error {
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
	openSession func() (libvirtstorage.Client, error),
	mutation *volumeMutation,
) (storage.StoragePoolVolumeStatusSpec, error) {
	pool, err := ctrl.resolvePool(ctx, r, logger, spec.Pool, poolStatuses, held)
	if err != nil {
		status := storage.StoragePoolVolumeStatusSpec{
			Phase: storage.StoragePoolVolumePhaseNotReady,
		}
		if observed := poolStatuses[spec.Pool]; observed != nil && observed.Metadata().Phase() == resource.PhaseRunning &&
			observed.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable {
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
	ctx context.Context, r controller.ReaderWriter, session libvirtstorage.Client, logger *zap.Logger,
	pool libvirtstorage.Pool, poolStatus *storage.StoragePoolStatus, spec *storage.StoragePoolVolumeSpecSpec,
	mutation *volumeMutation,
) (storage.StoragePoolVolumeStatusSpec, error) {
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

// resolvePool holds the pool a volume lives in and names it to libvirt, or says why it cannot yet.
func (ctrl *StoragePoolVolumeController) resolvePool(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	name string,
	poolStatuses map[string]*storage.StoragePoolStatus,
	held map[string]struct{},
) (libvirtstorage.Pool, error) {
	poolStatus, found := poolStatuses[name]
	if !found {
		return libvirtstorage.Pool{}, pending(
			"storage pool %q has no status yet: it is either not declared by a StoragePool document, or not reconciled",
			name)
	}

	if poolStatus.TypedSpec().Phase != storage.StoragePoolPhaseReady {
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
	if err := recheckHeldPool(ctx, r, name, poolStatus); err != nil {
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

func recheckHeldPool(ctx context.Context, r controller.Reader, name string, poolStatus *storage.StoragePoolStatus, ownMarkers ...string) error {
	current, err := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, name)
	if err != nil {
		return fmt.Errorf("error rechecking storage pool %q: %w", name, err)
	}

	if current.Metadata().Phase() != resource.PhaseRunning || current.TypedSpec().Phase != storage.StoragePoolPhaseReady ||
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
	session libvirtstorage.Client,
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
	session libvirtstorage.Client,
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

	if err := recheckHeldPool(ctx, r, spec.Pool, poolStatus, marker); err != nil {
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
	session libvirtstorage.Client, pool libvirtstorage.Pool, spec *storage.StoragePoolVolumeSpecSpec, mutation *volumeMutation,
) error {
	operation, ok := session.(interface {
		ResizeVolumeOperation(libvirtstorage.Pool, string, uint64) (libvirtstorage.OperationOutcome, error)
	})
	if !ok {
		// No RPC was invoked: this pass may retire its own marker, but must
		// report an error rather than falling back to the legacy mutation API.
		return fmt.Errorf("storage client does not provide resize operation outcome evidence")
	}

	// Fail closed even if the client returns a malformed outcome or Unknown with nil.
	mutation.outcome = libvirtstorage.Unknown

	outcome, err := operation.ResizeVolumeOperation(pool, spec.Name, spec.Capacity)
	if outcome == libvirtstorage.NoMutationSubmitted || outcome == libvirtstorage.Finished {
		mutation.outcome = outcome
	}

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
