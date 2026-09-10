// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/md"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block/blockhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const minMembers = 2

// MDProvisioner is the reconciler's mdadm subset.
type MDProvisioner interface {
	Create(ctx context.Context, name string, opts md.CreateOptions) (string, error)
	Add(ctx context.Context, device string, members ...string) error
	Grow(ctx context.Context, device string, raidDevices int) error
	DetailDevice(ctx context.Context, device string) (md.Detail, error)
	FindDeviceByMember(member string) (string, error)
	ListArrays() ([]string, error)
	IsSyncing(device string) (bool, error)
	ArrayStateForDevice(device string) (string, error)
	SyncActionForDevice(device string) (md.SyncAction, error)
}

// MDArrayReconcileController converges MDArraySpec resources into running MD arrays.
type MDArrayReconcileController struct {
	V1Alpha1Mode machineruntime.Mode
	State        state.State
	MD           MDProvisioner
}

// Name implements controller.Controller.
func (ctrl *MDArrayReconcileController) Name() string {
	return "storage.MDArrayReconcileController"
}

// Inputs implements controller.Controller.
func (ctrl *MDArrayReconcileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: storage.NamespaceName,
			Type:      storage.MDArraySpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiskType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.MDRefreshRequestType,
			ID:        optional.Some(storage.RefreshID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.SystemDiskType,
			ID:        optional.Some(block.SystemDiskID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller.
func (ctrl *MDArrayReconcileController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.MDArrayStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller.
func (ctrl *MDArrayReconcileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}
	}
}

func (ctrl *MDArrayReconcileController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	specs, err := safe.ReaderListAll[*storage.MDArraySpec](ctx, r)
	if err != nil {
		return fmt.Errorf("list MDArraySpec: %w", err)
	}

	r.StartTrackingOutputs()

	var reconcileErrs error

	claimed := map[string]struct{}{}
	specIDs := map[string]struct{}{}

	for spec := range specs.All() {
		id := spec.Metadata().ID()
		specIDs[id] = struct{}{}

		status, device, err := ctrl.reconcileArray(ctx, logger, id, spec.TypedSpec())
		if err != nil {
			if errors.Is(err, md.ErrResync) {
				logger.Debug("MD array is syncing; waiting for monitor refresh", zap.String("array", id), zap.Error(err))
			} else {
				reconcileErrs = errors.Join(reconcileErrs, fmt.Errorf("reconcile array %q: %w", id, err))
			}
		}

		if device != "" {
			claimed[device] = struct{}{}
		}

		if err := safe.WriterModify(ctx, r, storage.NewMDArrayStatus(storage.NamespaceName, id), func(s *storage.MDArrayStatus) error {
			*s.TypedSpec() = *status

			return nil
		}); err != nil {
			return fmt.Errorf("modify MDArrayStatus %q: %w", id, err)
		}
	}

	if err := ctrl.reportUnmanagedArrays(ctx, r, logger, claimed, specIDs); err != nil {
		return err
	}

	if err := safe.CleanupOutputs[*storage.MDArrayStatus](ctx, r); err != nil {
		return fmt.Errorf("cleanup MDArrayStatus outputs: %w", err)
	}

	if reconcileErrs != nil {
		logger.Warn("MD reconcile encountered errors", zap.Error(reconcileErrs))
	}

	return nil
}

// reportUnmanagedArrays reports MDArrayStatus for MD arrays present on the box which have
// no matching MDArraySpec (and therefore no RAIDArrayConfig), e.g. a boot mirror assembled
// before Talos ever wrote a machine config for it. These are observed only: no Create/Add/
// Grow is ever issued for them, and only raid1 arrays are reported, since that is the only
// level MDArrayStatusSpec.Level can represent today.
func (ctrl *MDArrayReconcileController) reportUnmanagedArrays(ctx context.Context, r controller.Runtime, logger *zap.Logger, claimed, specIDs map[string]struct{}) error {
	arrays, err := ctrl.MD.ListArrays()
	if err != nil {
		return fmt.Errorf("list MD arrays: %w", err)
	}

	for _, device := range arrays {
		if _, ok := claimed[device]; ok {
			continue
		}

		id := filepath.Base(device)
		if _, ok := specIDs[id]; ok {
			continue
		}

		detail, err := ctrl.MD.DetailDevice(ctx, device)
		if err != nil {
			// e.g. an assembled-but-not-yet-running array; nothing trustworthy to report
			// until it can be queried (MDLastResortController will run it if it's stuck).
			continue
		}

		if detail.Level != "" && detail.Level != "raid1" {
			// storage.MDLevel only models raid1 today: report nothing rather than mislabel
			// the level of an array Talos doesn't actually know how to represent.
			logger.Debug("skipping unmanaged array with unsupported level",
				zap.String("device", device), zap.String("level", detail.Level))

			continue
		}

		status := &storage.MDArrayStatusSpec{Level: storage.MDLevelRAID1, Device: device, Unmanaged: true}

		ctrl.updateObservedStatus(ctx, device, status)
		markReadyIfIdle(status)

		if err := safe.WriterModify(ctx, r, storage.NewMDArrayStatus(storage.NamespaceName, id), func(s *storage.MDArrayStatus) error {
			*s.TypedSpec() = *status

			return nil
		}); err != nil {
			return fmt.Errorf("modify MDArrayStatus %q: %w", id, err)
		}
	}

	return nil
}

func (ctrl *MDArrayReconcileController) reconcileArray(ctx context.Context, logger *zap.Logger, name string, spec *storage.MDArraySpecSpec) (*storage.MDArrayStatusSpec, string, error) {
	status := mdArrayStatusForSpec(name, spec)

	diskPaths, err := ctrl.matchMembers(ctx, &spec.VolumeSelector)
	if err != nil {
		errStatus, wrapErr := statusWithError(status, fmt.Errorf("match disks: %w", err))

		return errStatus, "", wrapErr
	}

	status.Members = diskPaths

	if len(diskPaths) < minMembers {
		logger.Debug("waiting for enough member disks",
			zap.String("array", name),
			zap.Int("matched", len(diskPaths)),
			zap.Int("required", minMembers))

		status.Status = storage.MDArrayPhaseWaiting
		status.Error = waitingForMembersError(diskPaths)

		return status, "", nil
	}

	device, err := ctrl.findExistingDevice(diskPaths)
	if err != nil {
		errStatus, wrapErr := statusWithError(status, fmt.Errorf("find existing MD device: %w", err))

		return errStatus, "", wrapErr
	}

	if device == "" {
		return ctrl.createArray(ctx, logger, name, spec, diskPaths, status)
	}

	if err := ctrl.reconcileExistingArray(ctx, logger, name, device, diskPaths); err != nil {
		ctrl.updateObservedStatus(ctx, device, status)

		if errors.Is(err, md.ErrResync) {
			status.Status = storage.MDArrayPhaseRebuilding
		} else {
			status.Status = storage.MDArrayPhaseError
		}

		status.Error = provisioningError(err)

		return status, device, err
	}

	ctrl.updateObservedStatus(ctx, device, status)
	markReadyIfIdle(status)

	return status, device, nil
}

func mdArrayStatusForSpec(name string, spec *storage.MDArraySpecSpec) *storage.MDArrayStatusSpec {
	return &storage.MDArrayStatusSpec{Level: spec.Level, Device: md.DevicePath(name)}
}

func waitingForMembersError(members []string) string {
	return fmt.Sprintf("waiting for enough member disks: matched %d, required %d", len(members), minMembers)
}

func statusWithError(status *storage.MDArrayStatusSpec, err error) (*storage.MDArrayStatusSpec, error) {
	status.Status = storage.MDArrayPhaseError
	status.Error = provisioningError(err)

	return status, err
}

func markReadyIfIdle(status *storage.MDArrayStatusSpec) {
	if status.Status != storage.MDArrayPhaseRebuilding {
		status.Status = storage.MDArrayPhaseReady
	}
}

func (ctrl *MDArrayReconcileController) createArray(
	ctx context.Context,
	logger *zap.Logger,
	name string,
	spec *storage.MDArraySpecSpec,
	members []string,
	status *storage.MDArrayStatusSpec,
) (*storage.MDArrayStatusSpec, string, error) {
	logger.Info("creating MD array", zap.String("array", name), zap.Strings("members", members))

	device, err := ctrl.MD.Create(ctx, name, md.CreateOptions{Level: spec.Level.Mdadm(), Metadata: spec.Metadata.Mdadm(), RaidDevices: len(members), Devices: members})
	if err != nil && !errors.Is(err, md.ErrExists) {
		errStatus, wrapErr := statusWithError(status, fmt.Errorf("create: %w", err))

		return errStatus, "", wrapErr
	}

	if device == "" {
		var findErr error

		device, findErr = ctrl.findExistingDevice(members)
		if findErr != nil {
			errStatus, wrapErr := statusWithError(status, fmt.Errorf("find existing MD device: %w", findErr))

			return errStatus, "", wrapErr
		}
	}

	if device == "" {
		status.Status = storage.MDArrayPhaseError
		status.Error = "array reported as existing but device could not be resolved (may be inactive)"

		return status, "", nil
	}

	ctrl.updateObservedStatus(ctx, device, status)
	markReadyIfIdle(status)

	return status, device, nil
}

func (ctrl *MDArrayReconcileController) reconcileExistingArray(ctx context.Context, logger *zap.Logger, name, device string, desiredMembers []string) error {
	detail, err := ctrl.MD.DetailDevice(ctx, device)
	if err != nil {
		return fmt.Errorf("detail: %w", err)
	}

	toAdd := membersToAdd(desiredMembers, detail.Members)

	raidDevices := targetRaidDevices(detail, toAdd)
	if !needsArrayReconcile(detail, toAdd, raidDevices) {
		return nil
	}

	if err := ctrl.ensureArrayIdle(device); err != nil {
		return err
	}

	logger.Info("extending MD array", zap.String("array", name), zap.Strings("members", toAdd), zap.Int("raid_devices", raidDevices))

	if err := ctrl.addMissingMembers(ctx, device, toAdd); err != nil {
		return err
	}

	if err := ctrl.growArray(ctx, device, detail, raidDevices); err != nil {
		return err
	}

	if err := triggerBlockDeviceChange(device); err != nil {
		logger.Debug("failed to trigger block device change", zap.String("device", device), zap.Error(err))
	}

	return nil
}

func needsArrayReconcile(detail md.Detail, toAdd []string, raidDevices int) bool {
	return len(toAdd) > 0 || detail.RaidDevices < raidDevices
}

func (ctrl *MDArrayReconcileController) addMissingMembers(ctx context.Context, device string, toAdd []string) error {
	if len(toAdd) == 0 {
		return nil
	}

	if err := ctrl.MD.Add(ctx, device, toAdd...); err != nil && !errors.Is(err, md.ErrExists) {
		return fmt.Errorf("add: %w", err)
	}

	return nil
}

func (ctrl *MDArrayReconcileController) growArray(ctx context.Context, device string, detail md.Detail, raidDevices int) error {
	if detail.RaidDevices >= raidDevices {
		return nil
	}

	if err := ctrl.MD.Grow(ctx, device, raidDevices); err != nil {
		return fmt.Errorf("grow: %w", err)
	}

	return nil
}

func targetRaidDevices(detail md.Detail, toAdd []string) int {
	return len(detail.Members) + len(toAdd)
}

func (ctrl *MDArrayReconcileController) ensureArrayIdle(device string) error {
	busy, err := ctrl.MD.IsSyncing(device)
	if err != nil {
		return fmt.Errorf("check sync status: %w", err)
	}

	if busy {
		return fmt.Errorf("waiting for MD array sync to finish: %w", md.ErrResync)
	}

	return nil
}

func (ctrl *MDArrayReconcileController) updateObservedStatus(ctx context.Context, device string, status *storage.MDArrayStatusSpec) {
	if detail, err := ctrl.MD.DetailDevice(ctx, device); err == nil {
		status.Members = detail.Members
		status.RaidDevices = detail.RaidDevices
		status.UUID = detail.UUID
		status.Name = detail.Name
		status.Metadata = detail.Metadata
	}

	if arrayState, err := ctrl.MD.ArrayStateForDevice(device); err == nil {
		status.ArrayState = arrayState
	}

	if syncAction, err := ctrl.MD.SyncActionForDevice(device); err == nil {
		status.SyncAction = string(syncAction)
		if syncAction != "" && syncAction != md.SyncActionIdle {
			status.Status = storage.MDArrayPhaseRebuilding
		}
	}
}

func (ctrl *MDArrayReconcileController) matchMembers(ctx context.Context, selector *cel.Expression) ([]string, error) {
	disks, err := safe.StateListAll[*block.Disk](ctx, ctrl.State)
	if err != nil {
		return nil, fmt.Errorf("list disks: %w", err)
	}

	volumes, err := safe.StateListAll[*block.DiscoveredVolume](ctx, ctrl.State)
	if err != nil {
		return nil, fmt.Errorf("list discovered volumes: %w", err)
	}

	systemDiskDevPath := ""

	systemDisk, err := safe.StateGetByID[*block.SystemDisk](ctx, ctrl.State, block.SystemDiskID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("get system disk: %w", err)
	}

	if systemDisk != nil {
		systemDiskDevPath = systemDisk.TypedSpec().DevPath
	}

	contexts, err := blockhelpers.BuildMatchContexts(slices.Collect(disks.All()), slices.Collect(volumes.All()), systemDiskDevPath)
	if err != nil {
		return nil, err
	}

	return matchingMemberDevices(contexts, selector)
}

func matchingMemberDevices(contexts []blockhelpers.MatchContext, selector *cel.Expression) ([]string, error) {
	var diskPaths []string

	for _, c := range contexts {
		if c.Partitioned || c.SystemDisk {
			continue
		}

		matches, err := selector.EvalBool(celenv.VolumeLocator(), c.CELContext)
		if err != nil {
			return nil, fmt.Errorf("evaluate selector: %w", err)
		}

		if matches {
			diskPaths = append(diskPaths, c.DevPath)
		}
	}

	sort.Strings(diskPaths)

	return diskPaths, nil
}

func triggerBlockDeviceChange(device string) error {
	return os.WriteFile(filepath.Join("/sys/class/block", filepath.Base(device), "uevent"), []byte("change\n"), 0o644)
}

func provisioningError(err error) string {
	message := err.Error()

	if execErr, ok := errors.AsType[*md.ExecError](err); ok {
		if stderr := strings.TrimSpace(string(execErr.Stderr)); stderr != "" {
			message += ": " + stderr
		}
	}

	return message
}

func (ctrl *MDArrayReconcileController) findExistingDevice(diskPaths []string) (string, error) {
	for _, member := range diskPaths {
		node, err := ctrl.MD.FindDeviceByMember(member)
		if err == nil {
			return node, nil
		}

		if !errors.Is(err, md.ErrNotFound) {
			return "", err
		}
	}

	return "", nil
}

func membersToAdd(diskPaths, existingMembers []string) []string {
	existing := make(map[string]struct{}, len(existingMembers))
	for _, m := range existingMembers {
		existing[m] = struct{}{}
	}

	var toAdd []string

	for _, disk := range diskPaths {
		if _, ok := existing[disk]; !ok {
			toAdd = append(toAdd, disk)
		}
	}

	return toAdd
}
