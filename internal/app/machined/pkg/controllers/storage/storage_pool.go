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

	"github.com/siderolabs/talos/internal/pkg/libvirtstorage"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// errPoolPending marks conditions resolved by an input event, not by restart backoff.
var errPoolPending = errors.New("pool pending")

// StoragePoolController defines libvirt directory pools on existing volume mounts.
//
// Like other mount consumers, it holds a finalizer on the VolumeMountStatus while
// the pool uses it and releases the hold once the pool is stopped.
type StoragePoolController struct {
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
	if ctrl.Open == nil {
		ctrl.Open = libvirtstorage.Open
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

		client, err := ctrl.Open(ctx)
		if err != nil {
			// The daemon stops before volumes are finalized on shutdown. Without it no
			// pool is running, so a tearing-down mount can be released right away;
			// otherwise volume teardown would block until its deadline.
			if err = ctrl.releaseTearingDown(ctx, r, mounts); err != nil {
				return err
			}

			if specs.Len() == 0 {
				// Nothing to do without a daemon; don't restart-loop while unused.
				continue
			}

			// Daemon recovery emits no resource event: report and retry via backoff.
			if err = ctrl.reportUnavailable(ctx, r, specs, fmt.Errorf("waiting for storage daemon: %w", err)); err != nil {
				return err
			}

			return fmt.Errorf("waiting for storage daemon")
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
) error {
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

	for _, pool := range pools {
		if _, wanted := desired[pool.Name]; wanted || pool.UUID != libvirtstorage.UUID(machineUUID, pool.Name) {
			continue
		}

		if err = client.Remove(pool); err != nil {
			return fmt.Errorf("error removing storage pool %q: %w", pool.Name, err)
		}
	}

	// Release holds on mounts which are no longer usable or no longer referenced.
	// Pools defined on such a mount are stopped first so libvirt never uses a
	// mount we do not hold; activation below restarts the desired ones.
	for mount := range mounts.All() {
		if !mount.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if slices.Contains(slices.Collect(maps.Values(desired)), mount.TypedSpec().VolumeID) && mountWritable(mount) {
			continue
		}

		// Match on the libvirt definition's target, not the spec: a retargeted pool
		// still points at the old mount until Ensure redefines it.
		for _, pool := range pools {
			if pool.UUID != libvirtstorage.UUID(machineUUID, pool.Name) || !pathUnder(pool.Target, mount.TypedSpec().Target) {
				continue
			}

			if err = client.Stop(pool); err != nil {
				return fmt.Errorf("error stopping storage pool %q: %w", pool.Name, err)
			}
		}

		if err = r.RemoveFinalizer(ctx, mount.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer from volume mount status %q: %w", mount.Metadata().ID(), err)
		}
	}

	r.StartTrackingOutputs()

	var activateErrors error

	for name, volumeID := range desired {
		pool := libvirtstorage.Pool{Name: name, UUID: libvirtstorage.UUID(machineUUID, name)}

		target, activateErr := ctrl.activate(ctx, r, client, pool, volumeID)
		if activateErr != nil && !errors.Is(activateErr, errPoolPending) {
			// libvirt/filesystem failures emit no resource event: retry via backoff.
			activateErrors = errors.Join(activateErrors, fmt.Errorf("pool %q: %w", name, activateErr))
		}

		if err = safe.WriterModify(ctx, r, storage.NewStoragePoolStatus(storage.NamespaceName, name), func(status *storage.StoragePoolStatus) error {
			*status.TypedSpec() = storage.StoragePoolStatusSpec{
				VolumeID:   volumeID,
				TargetPath: target,
				Ready:      activateErr == nil,
			}

			if activateErr != nil {
				status.TypedSpec().Error = activateErr.Error()
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating storage pool status %q: %w", name, err)
		}
	}

	if err = safe.CleanupOutputs[*storage.StoragePoolStatus](ctx, r); err != nil {
		return err
	}

	return activateErrors
}

// activate exposes the pool once its backing mount is held.
//
// A pending mount is reported through the status wrapped in errPoolPending, not
// retried: the mount status input wakes the controller when it changes.
func (ctrl *StoragePoolController) activate(ctx context.Context, r controller.Runtime, client libvirtstorage.Client,
	pool libvirtstorage.Pool, volumeID string,
) (string, error) {
	mount, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, volumeID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", fmt.Errorf("waiting for backing volume %q to be mounted: %w", volumeID, errPoolPending)
		}

		return "", fmt.Errorf("error getting volume mount status: %w", err)
	}

	if !mountWritable(mount) {
		return "", fmt.Errorf("backing volume %q is not mounted for writing: %w", volumeID, errPoolPending)
	}

	if !mount.Metadata().Finalizers().Has(ctrl.Name()) {
		if err = r.AddFinalizer(ctx, mount.Metadata(), ctrl.Name()); err != nil {
			return "", fmt.Errorf("error adding finalizer to volume mount status: %w", err)
		}
	}

	target := filepath.Join(mount.TypedSpec().Target, pool.Name)

	if err = client.Ensure(pool, target, func() error { return preparePoolDirectory(target) }); err != nil {
		return "", err
	}

	return target, nil
}

// releaseTearingDown drops our hold on mounts being torn down while the daemon is unavailable.
func (ctrl *StoragePoolController) releaseTearingDown(ctx context.Context, r controller.Runtime, mounts safe.List[*block.VolumeMountStatus]) error {
	for mount := range mounts.All() {
		if mount.Metadata().Phase() != resource.PhaseTearingDown || !mount.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, mount.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer from volume mount status %q: %w", mount.Metadata().ID(), err)
		}
	}

	return nil
}

// reportUnavailable writes the daemon error to every desired pool status.
func (ctrl *StoragePoolController) reportUnavailable(ctx context.Context, r controller.Runtime, specs safe.List[*storage.StoragePoolSpec], reason error) error {
	r.StartTrackingOutputs()

	for spec := range specs.All() {
		if spec.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		if err := safe.WriterModify(ctx, r, storage.NewStoragePoolStatus(storage.NamespaceName, spec.Metadata().ID()), func(status *storage.StoragePoolStatus) error {
			*status.TypedSpec() = storage.StoragePoolStatusSpec{
				VolumeID: spec.TypedSpec().VolumeID,
				Error:    reason.Error(),
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating storage pool status %q: %w", spec.Metadata().ID(), err)
		}
	}

	return safe.CleanupOutputs[*storage.StoragePoolStatus](ctx, r)
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
