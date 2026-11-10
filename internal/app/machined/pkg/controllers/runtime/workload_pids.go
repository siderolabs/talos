// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// WorkloadPidsRoot is a root cgroup whose pids.max is enforced by WorkloadPidsController.
type WorkloadPidsRoot struct {
	// Cgroup is the cgroup name, relative to the Talos cgroup root (e.g. constants.CgroupTalosContainersRoot).
	Cgroup string

	// ReservedPids is the number of PIDs held back from the root.
	ReservedPids uint64
}

// WorkloadPidsController enforces pids.max on a set of workload root cgroups.
//
// The limit of each root is min(kernel.pid_max, kernel.threads-max) minus the root's reservation,
// floored at zero: the calculation the kubelet applies to kubepods. Unlike the kubelet, which reads the
// sysctls once at startup, the limits are recomputed whenever a kernel parameter changes.
//
// The controller is the only writer of pids.max on its roots.
type WorkloadPidsController struct {
	V1Alpha1Mode machineruntime.Mode

	// Roots are the cgroups to enforce pids.max on.
	Roots []WorkloadPidsRoot

	// CgroupRoot is the cgroup2 mount point, defaults to constants.CgroupMountPath. Overridable for testing.
	CgroupRoot string

	// ProcSysRoot is the sysctl mount point, defaults to constants.ProcSysPath. Overridable for testing.
	ProcSysRoot string
}

// Name implements controller.Controller interface.
func (ctrl *WorkloadPidsController) Name() string {
	return "runtime.WorkloadPidsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *WorkloadPidsController) Inputs() []controller.Input {
	return []controller.Input{
		// kernel.pid_max and kernel.threads-max are set via KernelParamSpecController
		// (Talos defaults, machine.sysctls); its status output signals that the capacity may have changed.
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.KernelParamStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *WorkloadPidsController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *WorkloadPidsController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	if len(ctrl.Roots) == 0 {
		return errors.New("no cgroup roots configured")
	}

	if ctrl.CgroupRoot == "" {
		ctrl.CgroupRoot = constants.CgroupMountPath
	}

	if ctrl.ProcSysRoot == "" {
		ctrl.ProcSysRoot = constants.ProcSysPath
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *WorkloadPidsController) reconcile(logger *zap.Logger) error {
	capacity, err := cgroup.PidCapacity(ctrl.ProcSysRoot)
	if err != nil {
		return fmt.Errorf("error reading PID capacity: %w", err)
	}

	var errs error

	for _, root := range ctrl.Roots {
		if err = ctrl.enforce(logger, root, capacity); err != nil {
			errs = errors.Join(errs, fmt.Errorf("error enforcing pids limit on cgroup %q: %w", root.Cgroup, err))
		}
	}

	return errs
}

func (ctrl *WorkloadPidsController) enforce(logger *zap.Logger, root WorkloadPidsRoot, capacity uint64) error {
	limit := cgroup.AllocatablePids(capacity, root.ReservedPids)
	if limit == 0 {
		logger.Warn(
			"PID capacity does not exceed the reservation, no new tasks can start in the cgroup",
			zap.String("cgroup", root.Cgroup),
			zap.Uint64("capacity", capacity),
			zap.Uint64("reserved", root.ReservedPids),
		)
	}

	desired := cgroup.LimitedPidsMax(limit)

	previous, written, err := cgroup.EnsurePidsMax(filepath.Join(ctrl.CgroupRoot, cgroup.Path(root.Cgroup)), desired)
	if err != nil {
		return err
	}

	if written {
		logger.Info(
			"pids limit updated",
			zap.String("cgroup", root.Cgroup),
			zap.Stringer("previous", previous),
			zap.Stringer("limit", desired),
			zap.Uint64("capacity", capacity),
		)
	}

	return nil
}
