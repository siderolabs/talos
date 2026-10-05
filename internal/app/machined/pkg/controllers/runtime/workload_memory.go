// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// WorkloadMemoryController enforces WorkloadMemorySpec on the Talos-owned workload cgroup roots.
//
// Kubelet owns kubepods. A missing spec is pending; zero limits in a present spec restore unlimited.
type WorkloadMemoryController struct {
	V1Alpha1Mode machineruntime.Mode

	// CgroupRoot is the cgroup2 mount point, defaults to constants.CgroupMountPath. Overridable for testing.
	CgroupRoot string
}

// Name implements controller.Controller interface.
func (ctrl *WorkloadMemoryController) Name() string {
	return "runtime.WorkloadMemoryController"
}

// Inputs implements controller.Controller interface.
func (ctrl *WorkloadMemoryController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.WorkloadMemorySpecType,
			ID:        optional.Some(runtime.WorkloadMemorySpecID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *WorkloadMemoryController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *WorkloadMemoryController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	if ctrl.CgroupRoot == "" {
		ctrl.CgroupRoot = constants.CgroupMountPath
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

		r.ResetRestartBackoff()
	}
}

func (ctrl *WorkloadMemoryController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	spec, err := safe.ReaderGetByID[*runtime.WorkloadMemorySpec](ctx, r, runtime.WorkloadMemorySpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error getting workload memory spec: %w", err)
	}

	var errs error

	for _, root := range []struct {
		name  string
		limit uint64
	}{
		{constants.CgroupTalosContainersRoot, spec.TypedSpec().TalosContainersLimit},
		{constants.CgroupVirtualMachines, spec.TypedSpec().VirtualMachinesLimit},
	} {
		if err = ctrl.enforce(logger, root.name, root.limit); err != nil {
			errs = errors.Join(errs, fmt.Errorf("error enforcing memory limit on cgroup %q: %w", root.name, err))
		}
	}

	return errs
}

func (ctrl *WorkloadMemoryController) enforce(logger *zap.Logger, name string, limit uint64) error {
	desired := cgroup.UnlimitedMemoryMax()

	if limit != 0 {
		normalized, err := kernel.NormalizeMemoryLimit(limit, os.Getpagesize())
		if err != nil {
			return err
		}

		desired = cgroup.LimitedMemoryMax(normalized)
	}

	previous, written, err := cgroup.EnsureMemoryMax(filepath.Join(ctrl.CgroupRoot, cgroup.Path(name)), desired)
	if err != nil {
		return err
	}

	if written {
		logger.Info(
			"memory limit updated",
			zap.String("cgroup", name),
			zap.Stringer("previous", previous),
			zap.Stringer("limit", desired),
		)
	}

	return nil
}
