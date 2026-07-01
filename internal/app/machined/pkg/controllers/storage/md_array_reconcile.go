// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/md"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block/blockhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// minMembers is the minimum member count for the only supported level (raid1).
const minMembers = 2

// MDProvisioner is the reconciler's mdadm subset.
type MDProvisioner interface {
	Create(ctx context.Context, name string, level, raidDevices int, devices []string) (string, error)
	Extend(ctx context.Context, name string, devices []string) error
	Detail(ctx context.Context, name string) (md.Detail, error)
}

// MDArrayReconcileController converges each MDArraySpec into a running MD array.
//
// Additive only: it creates arrays and adds members, never destroys. Removal
// goes through the MDService wipe RPCs (talosctl wipe md / wipe md-member).
type MDArrayReconcileController struct {
	V1Alpha1Mode machineruntime.Mode
	State        state.State
	MD           MDProvisioner
}

// Name implements controller.Controller interface.
func (ctrl *MDArrayReconcileController) Name() string {
	return "storage.MDArrayReconcileController"
}

// Inputs implements controller.Controller interface.
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
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MDArrayReconcileController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.MDArrayStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *MDArrayReconcileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// in container mode, no devices, nothing to provision
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	if ctrl.MD == nil {
		return errors.New("MD provisioner not configured")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		specs, err := safe.ReaderListAll[*storage.MDArraySpec](ctx, r)
		if err != nil {
			return fmt.Errorf("list MDArraySpec: %w", err)
		}

		r.StartTrackingOutputs()

		var reconcileErrs *multierror.Error

		for spec := range specs.All() {
			status, err := ctrl.reconcileArray(ctx, logger, spec.TypedSpec())
			if err != nil {
				reconcileErrs = multierror.Append(reconcileErrs, fmt.Errorf("reconcile array %q: %w", spec.TypedSpec().Name, err))

				continue
			}

			if status == nil {
				// Not enough members yet; park and re-run on the next block event.
				continue
			}

			if err := safe.WriterModify(
				ctx, r,
				storage.NewMDArrayStatus(storage.NamespaceName, status.Name),
				func(s *storage.MDArrayStatus) error {
					*s.TypedSpec() = *status

					return nil
				},
			); err != nil {
				return fmt.Errorf("modify MDArrayStatus %q: %w", status.Name, err)
			}
		}

		if err := safe.CleanupOutputs[*storage.MDArrayStatus](ctx, r); err != nil {
			return fmt.Errorf("cleanup MDArrayStatus outputs: %w", err)
		}

		if err := reconcileErrs.ErrorOrNil(); err != nil {
			logger.Warn("MD reconcile encountered errors", zap.Error(err))
		}
	}
}

// reconcileArray converges one array. Returns a nil status (no error) when the
// array cannot be provisioned yet (too few member disks matched).
//
//nolint:gocyclo
func (ctrl *MDArrayReconcileController) reconcileArray(ctx context.Context, logger *zap.Logger, spec *storage.MDArraySpecSpec) (*storage.MDArrayStatusSpec, error) {
	disks, err := blockhelpers.MatchDisks(ctx, ctrl.State, &spec.VolumeSelector)
	if err != nil {
		return nil, fmt.Errorf("match disks: %w", err)
	}

	diskPaths := xslices.Map(disks, func(d *block.Disk) string { return d.TypedSpec().DevPath })
	sort.Strings(diskPaths)

	if len(diskPaths) < minMembers {
		logger.Debug(
			"waiting for enough member disks",
			zap.String("array", spec.Name),
			zap.Int("matched", len(diskPaths)),
			zap.Int("required", minMembers),
		)

		return nil, nil
	}

	device := md.DevicePath(spec.Name)

	if _, statErr := os.Stat(device); statErr != nil {
		// ponytail: gate creation on the by-id device being absent only. The
		// array is built directly across the whole matched disks; mdadm refuses
		// a member that already carries a filesystem or foreign superblock, so
		// clearing stale disks is the operator's job (talosctl wipe), not the
		// reconcile's. Assembled arrays are adopted by the udev assembly rules,
		// which populate the by-id symlink before this runs.
		logger.Info(
			"creating MD array",
			zap.String("array", spec.Name),
			zap.Strings("members", diskPaths),
		)

		if _, err := ctrl.MD.Create(ctx, spec.Name, spec.Level.Mdadm(), len(diskPaths), diskPaths); err != nil && !errors.Is(err, md.ErrExists) {
			return nil, fmt.Errorf("create: %w", err)
		}
	} else {
		detail, err := ctrl.MD.Detail(ctx, spec.Name)
		if err != nil {
			return nil, fmt.Errorf("detail: %w", err)
		}

		toAdd := membersToAdd(diskPaths, detail.Members)
		if len(toAdd) > 0 {
			logger.Info(
				"extending MD array",
				zap.String("array", spec.Name),
				zap.Strings("members", toAdd),
			)

			if err := ctrl.MD.Extend(ctx, spec.Name, toAdd); err != nil && !errors.Is(err, md.ErrExists) {
				return nil, fmt.Errorf("extend: %w", err)
			}
		}
	}

	status := &storage.MDArrayStatusSpec{
		Name:   spec.Name,
		Level:  spec.Level,
		Device: device,
	}

	if detail, err := ctrl.MD.Detail(ctx, spec.Name); err == nil {
		status.Members = detail.Members
	}

	return status, nil
}

// membersToAdd returns the matched disks that are not yet members of the array.
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
