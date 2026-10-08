// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/cleanup"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/libvirt"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const poolMutationFinalizer = "storage.StoragePoolController/mutating/backing"

// errPoolPending marks conditions resolved by an input event, not by restart backoff.
var (
	errPoolPending      = errors.New("pool pending")
	errPoolRetargetHeld = errors.New("pool retarget held")
)

// StoragePoolController defines libvirt directory pools on existing volume mounts.
//
// Like other mount consumers, it holds a finalizer on the VolumeMountStatus while
// the pool uses it and releases the hold once the pool is stopped.
type StoragePoolController struct {
	V1Alpha1Mode machineruntime.Mode

	// Open is injectable for deterministic reconciliation tests.
	Open func(context.Context) (libvirtstorage.Client, error)
}

// Name implements controller.Controller interface.
func (ctrl *StoragePoolController) Name() string {
	return "storage.StoragePoolController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StoragePoolController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.SystemInformationType,
			ID:        optional.Some(hardware.SystemInformationID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolStatusType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StoragePoolController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.StoragePoolStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *StoragePoolController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	if ctrl.Open == nil {
		ctrl.Open = func(ctx context.Context) (libvirtstorage.Client, error) {
			return libvirt.New().Storage(ctx)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		specs, err := safe.ReaderListAll[*storage.StoragePoolSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing storage pool specs: %w", err)
		}

		mounts, err := safe.ReaderListAll[*block.VolumeMountStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing volume mount statuses: %w", err)
		}

		machineUUID, err := poolMachineUUID(ctx, r)
		if err != nil {
			return err
		}

		client, openErr := ctrl.Open(ctx)
		if openErr == nil {
			if _, ok := client.(libvirtstorage.EvidenceClient); !ok {
				client.Close()

				return fmt.Errorf("storage client does not provide mutation outcome evidence")
			}
		}

		if openErr != nil {
			// The daemon stops before volumes are finalized on shutdown, so a tearing-down mount
			// can be released right away rather than blocking until its deadline -- except under a
			// held pool, where a guest holds its disk open whether or not the daemon is up.
			//
			// Kept apart from openErr: this reported the wrong reason when it shared one variable,
			// and the daemon's own error is the only thing that says why it could not be reached.
			if err := ctrl.releaseTearingDown(ctx, r, mounts); err != nil {
				return err
			}

			if specs.Len() == 0 {
				// Nothing to do without a daemon; don't restart-loop while unused.
				continue
			}

			// Daemon recovery emits no resource event: report and retry via backoff.
			if err := ctrl.reportUnavailable(ctx, r, specs, fmt.Errorf("waiting for storage daemon: %w", openErr)); err != nil {
				return err
			}

			return fmt.Errorf("waiting for storage daemon: %w", openErr)
		}

		err = ctrl.reconcile(ctx, r, client, machineUUID, specs, mounts)

		client.Close()

		if err != nil {
			return err
		}
	}
}

//nolint:gocyclo,cyclop
func (ctrl *StoragePoolController) reconcile(ctx context.Context, r controller.Runtime, client libvirtstorage.Client,
	machineUUID uuid.UUID, specs safe.List[*storage.StoragePoolSpec], mounts safe.List[*block.VolumeMountStatus],
) (reconcileErr error) {
	desired := map[string]string{}

	for spec := range specs.All() {
		if spec.Metadata().Phase() == resource.PhaseRunning {
			desired[spec.Metadata().ID()] = spec.TypedSpec().VolumeID
		}
	}

	// Remove owned definitions which are no longer desired, including ones left
	// behind by a previous boot. Foreign pools are never touched.
	pools, err := client.Pools()
	if err != nil {
		return fmt.Errorf("error listing storage pools: %w", err)
	}

	statuses, err := safe.ReaderListAll[*storage.StoragePoolStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing storage pool statuses: %w", err)
	}

	// Inventory, status, and request withdrawal cannot prove that an operation
	// has stopped after its client session returned an error. An orphan marker
	// needs manual recovery after confirming the daemon is quiescent.
	uncertain := map[string]struct{}{}

	for status := range statuses.All() {
		if status.Metadata().Finalizers().Has(poolMutationFinalizer) {
			uncertain[status.Metadata().ID()] = struct{}{}
		}
	}

	// The preliminary census only permits deferral. Avoid marker churn while
	// a guest pins a retargeted or removed pool; a fresh census after publishing
	// exclusion is still required before any operation.
	alreadyHeld, err := heldPools(ctx, r)
	if err != nil {
		return err
	}

	// Publish exclusion before the fresh hold census. Consumers commit their
	// hold before rechecking this marker and the pool identity.
	evidence := client.(libvirtstorage.EvidenceClient)
	published := false
	pendingMounts := []*block.VolumeMountStatus{}
	// Only this pass's markers can be retired, and only after its status/cleanup
	// publication has succeeded. A returned RPC error alone is not evidence.
	changing := map[string]*storage.StoragePoolStatus{}
	failed := map[string]struct{}{}
	outcomes := map[string]libvirtstorage.OperationOutcome{}

	// Evidence only grows more restrictive during this pass. A later no-op
	// cannot undo partial progress or an unknown reply from an earlier call.
	recordOutcome := func(name string, outcome libvirtstorage.OperationOutcome) libvirtstorage.OperationOutcome {
		switch outcome {
		case libvirtstorage.NoMutationSubmitted, libvirtstorage.Finished, libvirtstorage.Unknown:
		default:
			outcome = libvirtstorage.Unknown
		}

		previous := outcomes[name]
		if previous == libvirtstorage.Unknown || (previous == libvirtstorage.Finished && outcome == libvirtstorage.NoMutationSubmitted) {
			outcome = previous
		}

		outcomes[name] = outcome
		if outcome == libvirtstorage.Unknown {
			failed[name] = struct{}{}
		}

		return outcome
	}

	defer func() {
		markerReleaseFailed := false

		for name, status := range changing {
			if _, ambiguous := failed[name]; ambiguous || (!published && outcomes[name] != libvirtstorage.NoMutationSubmitted) {
				continue
			}

			if releaseErr := r.RemoveFinalizer(ctx, status.Metadata(), poolMutationFinalizer); releaseErr != nil && !state.IsNotFoundError(releaseErr) {
				markerReleaseFailed = true
				reconcileErr = errors.Join(reconcileErr, fmt.Errorf("error releasing storage pool exclusion %q: %w", status.Metadata().ID(), releaseErr))
			}
		}

		if !published || markerReleaseFailed || len(failed) != 0 {
			return
		}

		for _, mount := range pendingMounts {
			if releaseErr := r.RemoveFinalizer(ctx, mount.Metadata(), ctrl.Name()); releaseErr != nil {
				reconcileErr = errors.Join(reconcileErr, fmt.Errorf("error removing finalizer from volume mount status %q: %w", mount.Metadata().ID(), releaseErr))
			}
		}
	}()

	for _, pool := range pools {
		volumeID, wanted := desired[pool.Name]
		if pool.UUID != libvirtstorage.UUID(machineUUID, pool.Name) {
			continue
		}

		if _, blocked := uncertain[pool.Name]; blocked {
			continue
		}

		status, getErr := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, pool.Name)
		if state.IsNotFoundError(getErr) {
			continue
		}

		if getErr != nil {
			return fmt.Errorf("error getting storage pool status %q: %w", pool.Name, getErr)
		}

		if wanted && status.TypedSpec().VolumeID == volumeID &&
			(status.TypedSpec().Phase == storage.StoragePoolPhaseReady || status.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable) &&
			pool.Target == status.TypedSpec().TargetPath && pool.Type == "dir" && pool.Active && pool.Persistent && !pool.Autostart {
			continue
		}

		if wanted {
			mount, mountErr := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, volumeID)
			if mountErr != nil && !state.IsNotFoundError(mountErr) {
				return fmt.Errorf("error getting backing mount %q: %w", volumeID, mountErr)
			}

			if mount == nil || !mountWritable(mount) {
				continue // activation cannot change the definition without a writable mount
			}
		}

		if _, heldBeforePublication := alreadyHeld[pool.Name]; heldBeforePublication {
			continue
		}

		if err = r.AddFinalizer(ctx, status.Metadata(), poolMutationFinalizer); err != nil {
			return fmt.Errorf("error excluding storage pool %q: %w", pool.Name, err)
		}

		changing[pool.Name] = status
	}

	// A pool whose status something still holds is one a guest may have a volume of open. Its
	// definition and its mount both stay until the hold comes back, which the teardown below asks
	// for.
	held, err := heldPools(ctx, r)
	if err != nil {
		return err
	}

	for name := range alreadyHeld {
		held[name] = struct{}{}
	}

	for name := range uncertain {
		held[name] = struct{}{}
	}

	var removalErrors error

	for _, pool := range pools {
		_, wanted := desired[pool.Name]
		if wanted || pool.UUID != libvirtstorage.UUID(machineUUID, pool.Name) {
			continue
		}

		if _, isHeld := held[pool.Name]; isHeld {
			continue
		}

		if err = excludePoolMutation(ctx, r, pool.Name, changing); err != nil {
			return err
		}

		freshHeld, censusErr := heldPools(ctx, r)
		if censusErr != nil {
			return censusErr
		}

		if _, isHeld := freshHeld[pool.Name]; isHeld {
			continue
		}

		outcome, removeErr := evidence.RemoveOperation(pool)

		outcome = recordOutcome(pool.Name, outcome)
		if outcome == libvirtstorage.Unknown {
			if removeErr == nil {
				removeErr = errors.New("removal outcome unknown")
			}
		}

		if removeErr != nil {
			removalErrors = errors.Join(removalErrors, fmt.Errorf("error removing storage pool %q: %w", pool.Name, removeErr))

			if outcome == libvirtstorage.Finished {
				if err = safe.WriterModify(ctx, r, storage.NewStoragePoolStatus(storage.NamespaceName, pool.Name), func(status *storage.StoragePoolStatus) error {
					status.TypedSpec().Phase = storage.StoragePoolPhaseNotReady
					status.TypedSpec().Error = removalErrors.Error()

					return nil
				}); err != nil {
					return fmt.Errorf("error publishing stopped storage pool %q: %w", pool.Name, err)
				}
			}

			break
		}
	}

	// Release holds on mounts which are no longer usable or no longer referenced.
	// Pools defined on such a mount are stopped first so libvirt never uses a
	// mount we do not hold; activation below restarts the desired ones.
	var stopErrors error

	stopped := map[string]struct{}{}

	for mount := range mounts.All() {
		if removalErrors != nil || stopErrors != nil {
			break
		}

		if !mount.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if slices.Contains(slices.Collect(maps.Values(desired)), mount.TypedSpec().VolumeID) && mountWritable(mount) {
			continue
		}

		// An uncertain retarget can still be using either backing mount even
		// when the old inventory or current spec names only one of them.
		if len(uncertain) != 0 {
			continue
		}

		// Match on the libvirt definition's target, not the spec: a retargeted pool
		// still points at the old mount until Ensure redefines it.
		if poolsHeldUnder(pools, held, machineUUID, mount.TypedSpec().Target) {
			// Releasing now would unmount the filesystem a guest is writing into. Stopping the pool
			// would not: a directory pool is metadata, and a domain opens its disks by absolute
			// path. The mount is the part that cannot be taken away, so neither happens until the
			// hold does come back.
			continue
		}

		canRelease := true

		for _, pool := range pools {
			if pool.UUID != libvirtstorage.UUID(machineUUID, pool.Name) || !pathUnder(pool.Target, mount.TypedSpec().Target) {
				continue
			}

			if _, blocked := uncertain[pool.Name]; blocked {
				continue
			}

			status, statusErr := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, pool.Name)
			if statusErr != nil && !state.IsNotFoundError(statusErr) {
				return fmt.Errorf("error getting storage pool status %q: %w", pool.Name, statusErr)
			}

			if statusErr == nil && status.Metadata().Phase() == resource.PhaseTearingDown {
				// COSI cannot install a new finalizer after teardown begins.
				// Without an earlier claim, keep the mount rather than submit an unfenced stop.
				canRelease = false

				continue
			}

			if err = excludePoolMutation(ctx, r, pool.Name, changing); err != nil {
				return err
			}

			freshHeld, censusErr := heldPools(ctx, r)
			if censusErr != nil {
				return censusErr
			}

			if _, isHeld := freshHeld[pool.Name]; isHeld {
				canRelease = false

				continue
			}

			outcome, stopErr := evidence.StopOperation(pool)

			outcome = recordOutcome(pool.Name, outcome)
			stopped[pool.Name] = struct{}{}

			if outcome == libvirtstorage.Unknown && stopErr == nil {
				stopErr = errors.New("stop outcome unknown")
			}

			if stopErr != nil {
				stopErrors = errors.Join(stopErrors, fmt.Errorf("error stopping storage pool %q: %w", pool.Name, stopErr))
				canRelease = false

				break
			}
		}

		if canRelease {
			pendingMounts = append(pendingMounts, mount)
		}
	}

	if stopErrors != nil {
		// Earlier successful stops also invalidated their old Ready observations.
		// Publish all of them before retiring any invocation-owned Finished marker.
		for name := range stopped {
			if err = safe.WriterModify(ctx, r, storage.NewStoragePoolStatus(storage.NamespaceName, name), func(status *storage.StoragePoolStatus) error {
				status.TypedSpec().Phase = storage.StoragePoolPhaseNotReady
				status.TypedSpec().Error = stopErrors.Error()

				return nil
			}); err != nil {
				return errors.Join(stopErrors, fmt.Errorf("error publishing stopped storage pool %q: %w", name, err))
			}
		}

		wanted := wantedPoolStatuses(desired)
		for name := range uncertain {
			wanted[name] = struct{}{}
		}

		for name := range changing {
			wanted[name] = struct{}{}
		}

		if err = cleanup.Outputs[*storage.StoragePoolStatus](ctx, r, "storage pool status", wanted); err != nil {
			return errors.Join(stopErrors, err)
		}

		published = true

		return errors.Join(removalErrors, stopErrors)
	}

	var activateErrors error

	for name, volumeID := range desired {
		pool := libvirtstorage.Pool{Name: name, UUID: libvirtstorage.UUID(machineUUID, name)}

		var (
			target      string
			activateErr error
		)

		// A volume consumer holds the old directory through a retarget. Do not redefine the
		// libvirt pool before that hold is returned: the new definition would hide the old
		// target from the mount-release guard on the following pass.
		if _, blocked := uncertain[name]; blocked {
			activateErr = fmt.Errorf("storage pool %q has an uncertain prior mutation; recovery requires confirming the daemon is quiescent before retiring its mutation marker: %w", name, errPoolPending)
		} else if _, isHeld := held[name]; isHeld {
			var previous *storage.StoragePoolStatus

			previous, activateErr = safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, name)
			if activateErr == nil && previous.TypedSpec().VolumeID != volumeID {
				activateErr = fmt.Errorf("pool is held by a consumer on volume %q: %w", previous.TypedSpec().VolumeID, errors.Join(errPoolPending, errPoolRetargetHeld))
			}
		}

		if activateErr == nil {
			// A confirmed Ready definition with the same backing needs its mount
			// hold refreshed, but not another daemon mutation (or marker churn).
			knownReadyTarget := ""

			if previous, readErr := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, name); readErr == nil &&
				previous.Metadata().Phase() == resource.PhaseRunning &&
				(previous.TypedSpec().Phase == storage.StoragePoolPhaseReady || previous.TypedSpec().Phase == storage.StoragePoolPhaseObservationUnavailable) &&
				previous.TypedSpec().VolumeID == volumeID {
				for _, existing := range pools {
					if existing.Name == name && existing.UUID == pool.UUID && existing.Target == previous.TypedSpec().TargetPath &&
						existing.Type == "dir" && existing.Active && existing.Persistent && !existing.Autostart {
						knownReadyTarget = existing.Target

						break
					}
				}
			}

			knownReady := false

			if knownReadyTarget != "" {
				if mount, mountErr := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, volumeID); mountErr == nil &&
					filepath.Join(mount.TypedSpec().Target, name) == knownReadyTarget {
					knownReady = true
				}
			}

			if !knownReady {
				mount, mountErr := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, volumeID)
				if mountErr != nil && !state.IsNotFoundError(mountErr) {
					return fmt.Errorf("error getting backing mount %q: %w", volumeID, mountErr)
				}

				switch {
				case mount == nil:
					activateErr = fmt.Errorf("waiting for backing volume %q to be mounted: %w", volumeID, errPoolPending)
				case !mountWritable(mount):
					activateErr = fmt.Errorf("backing volume %q is not mounted for writing: %w", volumeID, errPoolPending)
				default:
					if err = excludePoolMutation(ctx, r, name, changing); err != nil {
						return err
					}

					freshHeld, censusErr := heldPools(ctx, r)
					if censusErr != nil {
						return censusErr
					}

					if _, isHeld := freshHeld[name]; isHeld {
						activateErr = fmt.Errorf("pool is held by a consumer: %w", errPoolPending)
					}
				}
			}

			if activateErr == nil {
				var outcome libvirtstorage.OperationOutcome

				target, outcome, activateErr = ctrl.activate(ctx, r, evidence, pool, volumeID, knownReady)

				outcome = recordOutcome(name, outcome)
				if outcome == libvirtstorage.Unknown && activateErr == nil {
					activateErr = errors.New("ensure outcome unknown")
				}
			}
		}

		if activateErr != nil && !errors.Is(activateErr, errPoolPending) {
			// libvirt/filesystem failures emit no resource event: retry via backoff.
			activateErrors = errors.Join(activateErrors, fmt.Errorf("pool %q: %w", name, activateErr))
		}

		if _, blocked := uncertain[name]; blocked {
			current, readErr := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, name)
			if readErr != nil {
				return fmt.Errorf("error getting excluded storage pool status %q: %w", name, readErr)
			}

			if current.TypedSpec().VolumeID == volumeID && current.TypedSpec().Phase == storage.StoragePoolPhaseNotReady &&
				current.TypedSpec().Error == activateErr.Error() {
				continue
			}
		}

		if err = safe.WriterModify(ctx, r, storage.NewStoragePoolStatus(storage.NamespaceName, name), func(status *storage.StoragePoolStatus) error {
			if activateErr == nil || !errors.Is(activateErr, errPoolRetargetHeld) {
				status.TypedSpec().VolumeID = volumeID
			}

			status.TypedSpec().Phase = storage.StoragePoolPhaseNotReady
			if activateErr == nil {
				status.TypedSpec().Phase = storage.StoragePoolPhaseReady
			}

			status.TypedSpec().Error = ""

			// Kept rather than cleared when activation fails: TargetPath is the record of where the
			// pool was last put, and releaseTearingDown matches a tearing-down mount against it to
			// decide whether a guest may still be writing there. A failed activation clearing it --
			// which a mount beginning to tear down causes -- hands that mount straight back.
			// Nothing reads it while Ready is false.
			if target != "" {
				status.TypedSpec().TargetPath = target
			}

			if activateErr != nil {
				status.TypedSpec().Error = activateErr.Error()
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating storage pool status %q: %w", name, err)
		}
	}

	// Torn down rather than destroyed outright: a status a consumer holds cannot be destroyed, and
	// attempting it would fail this reconciliation for as long as the hold lasts.
	wanted := wantedPoolStatuses(desired)
	for name := range uncertain {
		wanted[name] = struct{}{}
	}

	for name := range changing {
		wanted[name] = struct{}{}
	}

	if err = cleanup.Outputs[*storage.StoragePoolStatus](ctx, r, "storage pool status", wanted); err != nil {
		return err
	}

	published = true

	return errors.Join(removalErrors, activateErrors)
}

// excludePoolMutation publishes admission exclusion before submitting a daemon
// operation. An orphan owned definition also needs a durable marker even though
// its status is initially absent.
func excludePoolMutation(ctx context.Context, r controller.Runtime, name string, changing map[string]*storage.StoragePoolStatus) error {
	if _, excluded := changing[name]; excluded {
		return nil
	}

	status := storage.NewStoragePoolStatus(storage.NamespaceName, name)
	if err := safe.WriterModify(ctx, r, status, func(*storage.StoragePoolStatus) error { return nil }); err != nil {
		return fmt.Errorf("error preparing storage pool exclusion %q: %w", name, err)
	}

	if err := r.AddFinalizer(ctx, status.Metadata(), poolMutationFinalizer); err != nil {
		return fmt.Errorf("error excluding storage pool %q: %w", name, err)
	}

	changing[name] = status

	return nil
}

// wantedPoolStatuses names the statuses the configuration still asks for.
func wantedPoolStatuses(desired map[string]string) map[resource.ID]struct{} {
	wanted := make(map[resource.ID]struct{}, len(desired))

	for name := range desired {
		wanted[name] = struct{}{}
	}

	return wanted
}

// heldPools names the pools whose status something still holds, by pool name.
//
// A hold means a consumer -- a volume of this pool, and through it a running guest -- is still
// relying on the pool's directory being where it is.
func heldPools(ctx context.Context, r controller.Reader) (map[string]struct{}, error) {
	statuses, err := safe.ReaderListAll[*storage.StoragePoolStatus](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing storage pool statuses: %w", err)
	}

	held := map[string]struct{}{}

	for status := range statuses.All() {
		if slices.ContainsFunc(*status.Metadata().Finalizers(), func(finalizer resource.Finalizer) bool {
			return finalizer != poolMutationFinalizer
		}) {
			held[status.Metadata().ID()] = struct{}{}
		}
	}

	// The volume controller may release its pool-status hold after the pool
	// becomes pending, while a VM acquires a hold on the still-Ready volume.
	// Count that consumer directly, including in the post-marker census.
	volumes, err := safe.ReaderListAll[*storage.StoragePoolVolumeStatus](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing storage pool volume statuses: %w", err)
	}

	for volume := range volumes.All() {
		if !volume.Metadata().Finalizers().Empty() && volume.TypedSpec().Pool != "" {
			held[volume.TypedSpec().Pool] = struct{}{}
		}
	}

	return held, nil
}

// poolsHeldUnder reports whether any held pool is defined under target.
func poolsHeldUnder(pools []libvirtstorage.Pool, held map[string]struct{}, machineUUID uuid.UUID, target string) bool {
	for _, pool := range pools {
		if pool.UUID != libvirtstorage.UUID(machineUUID, pool.Name) || !pathUnder(pool.Target, target) {
			continue
		}

		if _, isHeld := held[pool.Name]; isHeld {
			return true
		}
	}

	return false
}

// activate exposes the pool once its backing mount is held.
//
// A pending mount is reported through the status wrapped in errPoolPending, not
// retried: the mount status input wakes the controller when it changes.
func (ctrl *StoragePoolController) activate(ctx context.Context, r controller.Runtime, client libvirtstorage.EvidenceClient,
	pool libvirtstorage.Pool, volumeID string, knownReady bool,
) (string, libvirtstorage.OperationOutcome, error) {
	mount, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, volumeID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", libvirtstorage.NoMutationSubmitted, fmt.Errorf("waiting for backing volume %q to be mounted: %w", volumeID, errPoolPending)
		}

		return "", libvirtstorage.NoMutationSubmitted, fmt.Errorf("error getting volume mount status: %w", err)
	}

	if !mountWritable(mount) {
		return "", libvirtstorage.NoMutationSubmitted, fmt.Errorf("backing volume %q is not mounted for writing: %w", volumeID, errPoolPending)
	}

	if !mount.Metadata().Finalizers().Has(ctrl.Name()) {
		if err = r.AddFinalizer(ctx, mount.Metadata(), ctrl.Name()); err != nil {
			return "", libvirtstorage.NoMutationSubmitted, fmt.Errorf("error adding finalizer to volume mount status: %w", err)
		}
	}

	target := filepath.Join(mount.TypedSpec().Target, pool.Name)
	if knownReady {
		return target, libvirtstorage.NoMutationSubmitted, nil
	}

	outcome, err := client.EnsureOperation(pool, target, func() error { return preparePoolDirectory(target) })
	if err != nil {
		return "", outcome, err
	}

	return target, outcome, nil
}

// releaseTearingDown drops our hold on mounts being torn down while the daemon is unavailable.
//
// A mount under a held pool is kept, though. "No daemon means no pool is running, so nothing is
// writing" holds only for a pool whose contents are read through libvirt. A guest holds its disk
// open by file descriptor, so the storage daemon dying says nothing about whether the filesystem is
// busy -- and a daemon which merely crashed is not a shutdown.
func (ctrl *StoragePoolController) releaseTearingDown(ctx context.Context, r controller.Runtime, mounts safe.List[*block.VolumeMountStatus]) error {
	excluded, err := poolMutationExcluded(ctx, r)
	if err != nil {
		return err
	}

	if excluded {
		return nil
	}

	targets, err := heldPoolTargetsForOutage(ctx, r)
	if err != nil {
		return err
	}

	for mount := range mounts.All() {
		if mount.Metadata().Phase() != resource.PhaseTearingDown || !mount.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if mountUnderAny(mount.TypedSpec().Target, targets) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, mount.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer from volume mount status %q: %w", mount.Metadata().ID(), err)
		}
	}

	return nil
}

func heldPoolTargetsForOutage(ctx context.Context, r controller.Reader) (map[string]struct{}, error) {
	held, err := heldPools(ctx, r)
	if err != nil || len(held) == 0 {
		return nil, err
	}

	// Without a daemon the pools cannot be enumerated, so the status's own record of where the
	// pool was put is the only thing left to match a mount against.
	return heldPoolTargets(ctx, r, held)
}

// Without the daemon we cannot identify which backing mount an accepted but
// uncompleted retarget may touch. Keep every hold until quiesced recovery.
func poolMutationExcluded(ctx context.Context, r controller.Reader) (bool, error) {
	statuses, err := safe.ReaderListAll[*storage.StoragePoolStatus](ctx, r)
	if err != nil {
		return false, fmt.Errorf("error listing storage pool statuses: %w", err)
	}

	for status := range statuses.All() {
		if status.Metadata().Finalizers().Has(poolMutationFinalizer) {
			return true, nil
		}
	}

	return false, nil
}

// heldPoolTargets collects the directories of the held pools, as their statuses last reported them.
func heldPoolTargets(ctx context.Context, r controller.Reader, held map[string]struct{}) (map[string]struct{}, error) {
	statuses, err := safe.ReaderListAll[*storage.StoragePoolStatus](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing storage pool statuses: %w", err)
	}

	targets := map[string]struct{}{}

	for status := range statuses.All() {
		if _, isHeld := held[status.Metadata().ID()]; !isHeld {
			continue
		}

		if target := status.TypedSpec().TargetPath; target != "" {
			targets[target] = struct{}{}
		}
	}

	return targets, nil
}

// mountUnderAny reports whether any of the directories lies at or below the mount's target.
func mountUnderAny(mountTarget string, targets map[string]struct{}) bool {
	for target := range targets {
		if pathUnder(target, mountTarget) {
			return true
		}
	}

	return false
}

// reportUnavailable writes the daemon error to every desired pool status.
func (ctrl *StoragePoolController) reportUnavailable(ctx context.Context, r controller.Runtime, specs safe.List[*storage.StoragePoolSpec], reason error) error {
	wanted := map[resource.ID]struct{}{}

	held, err := heldPools(ctx, r)
	if err != nil {
		return err
	}

	for spec := range specs.All() {
		if spec.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		wanted[spec.Metadata().ID()] = struct{}{}

		_, isHeld := held[spec.Metadata().ID()]
		if err := reportPoolUnavailable(ctx, r, spec, isHeld, reason); err != nil {
			return fmt.Errorf("error updating storage pool status %q: %w", spec.Metadata().ID(), err)
		}
	}

	// An outage cannot establish completion of a submitted operation, even
	// when its spec has since been withdrawn.
	statuses, err := safe.ReaderListAll[*storage.StoragePoolStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing storage pool statuses: %w", err)
	}

	for status := range statuses.All() {
		if status.Metadata().Finalizers().Has(poolMutationFinalizer) {
			wanted[status.Metadata().ID()] = struct{}{}
		}
	}

	return cleanup.Outputs[*storage.StoragePoolStatus](ctx, r, "storage pool status", wanted)
}

// reportPoolUnavailable preserves observed identity but distinguishes backing
// withdrawal from observation loss. Consumers must not reconstruct pool intent.
func reportPoolUnavailable(ctx context.Context, r controller.ReaderWriter, spec *storage.StoragePoolSpec, held bool, reason error) error {
	mount, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, spec.TypedSpec().VolumeID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	return safe.WriterModify(ctx, r, storage.NewStoragePoolStatus(storage.NamespaceName, spec.Metadata().ID()), func(status *storage.StoragePoolStatus) error {
		// Preserve TargetPath and held VolumeID: with the daemon unreachable these
		// remain the only record of which mount an attached consumer uses.
		retarget := status.TypedSpec().VolumeID != "" && status.TypedSpec().VolumeID != spec.TypedSpec().VolumeID
		if !held || status.TypedSpec().VolumeID == "" {
			status.TypedSpec().VolumeID = spec.TypedSpec().VolumeID
		}

		status.TypedSpec().Phase = storage.StoragePoolPhaseObservationUnavailable
		if retarget || mount == nil || !mountWritable(mount) {
			status.TypedSpec().Phase = storage.StoragePoolPhaseNotReady
		}

		status.TypedSpec().Error = reason.Error()

		return nil
	})
}

// mountWritable is the single predicate for a mount a pool may be defined on.
func mountWritable(mount *block.VolumeMountStatus) bool {
	spec := mount.TypedSpec()

	return mount.Metadata().Phase() == resource.PhaseRunning && !spec.ReadOnly && !spec.Detached && filepath.IsAbs(spec.Target)
}

// pathUnder reports whether path is dir or lies below it.
func pathUnder(path, dir string) bool {
	if path == "" || dir == "" {
		return false
	}

	rel, err := filepath.Rel(dir, path)

	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

func poolMachineUUID(ctx context.Context, r controller.Reader) (uuid.UUID, error) {
	system, err := safe.ReaderGetByID[*hardware.SystemInformation](ctx, r, hardware.SystemInformationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("error getting system information: %w", err)
	}

	machineUUID, err := uuid.Parse(system.TypedSpec().UUID)
	if err != nil || machineUUID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid machine UUID %q", system.TypedSpec().UUID)
	}

	return machineUUID, nil
}

func preparePoolDirectory(target string) error {
	// Never MkdirAll: it could create an absent mount target on the rootfs.
	// Refuse symlinks so a pool cannot escape its backing volume.
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return os.Mkdir(target, 0o755)
	}

	if err != nil {
		return err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("pool target %q is not a directory", target)
	}

	return nil
}
