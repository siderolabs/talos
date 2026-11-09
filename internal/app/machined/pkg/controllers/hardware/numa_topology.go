// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware/internal/numa"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// NUMATopologyController publishes Linux NUMA inventory and watches hotplug events.
type NUMATopologyController struct {
	V1Alpha1Mode runtimetalos.Mode
	SysfsPath    string
	ProcfsPath   string
	// ReconcileCh triggers an additional reconcile for testing.
	ReconcileCh <-chan struct{}
}

// Name implements controller.Controller.
func (ctrl *NUMATopologyController) Name() string {
	return "hardware.NUMATopologyController"
}

// Inputs implements controller.Controller.
func (ctrl *NUMATopologyController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller.
func (ctrl *NUMATopologyController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.NUMATopologyType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller.
func (ctrl *NUMATopologyController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error { //nolint:gocyclo
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	if ctrl.SysfsPath == "" {
		ctrl.SysfsPath = "/sys"
	}

	if ctrl.ProcfsPath == "" {
		ctrl.ProcfsPath = procfs.DefaultMountPoint
	}

	watcher, err := kobject.NewWatcher(logger)
	if err != nil {
		return err
	}
	defer watcher.Close() //nolint:errcheck

	// The runtime reuses ctx across controller restarts, the trigger goroutine must not outlive Run.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := watcher.Run("cpu", "node", "memory")
	rateLimitedTrigger := trigger.NewRateLimitedTrigger(ctx, r, 1, 1)
	rateLimitedTrigger.QueueReconcile()

	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-watchCh:
			if !ok {
				select {
				case err := <-watcher.ErrCh():
					return err
				default:
					return nil
				}
			}

			rateLimitedTrigger.QueueReconcile()
		case err := <-watcher.ErrCh():
			return err
		case <-r.EventCh():
			if err := ctrl.reconcile(ctx, r, rateLimitedTrigger); err != nil {
				return err
			}

			r.ResetRestartBackoff()
		case <-ctrl.ReconcileCh:
			if err := ctrl.reconcile(ctx, r, rateLimitedTrigger); err != nil {
				return err
			}

			r.ResetRestartBackoff()
		}
	}
}

func (ctrl *NUMATopologyController) reconcile(ctx context.Context, r controller.Runtime, retry trigger.Trigger) error {
	spec, err := numa.Read(ctrl.SysfsPath, ctrl.ProcfsPath)
	if err != nil {
		// A failed or changing snapshot must not leave stale inventory available to consumers.
		if destroyErr := r.Destroy(ctx, hardware.NewNUMATopology().Metadata()); destroyErr != nil && !state.IsNotFoundError(destroyErr) {
			return errors.Join(err, destroyErr)
		}

		if errors.Is(err, numa.ErrTopologyChanged) {
			retry.QueueReconcile()

			return nil
		}

		return err
	}

	return safe.WriterModify(ctx, r, hardware.NewNUMATopology(), func(res *hardware.NUMATopology) error {
		*res.TypedSpec() = *spec

		return nil
	})
}
