// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// WorkloadResourceConfigController projects the Talos-owned roots of WorkloadResourceConfig into WorkloadMemorySpec.
//
// The kubepods limit travels with the kubelet configuration instead. The spec is published only once
// the machine configuration is active, so consumers can tell a pending projection (no resource) from
// no policy (zero limits). In container mode the projected policy is always empty.
type WorkloadResourceConfigController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *WorkloadResourceConfigController) Name() string {
	return "runtime.WorkloadResourceConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *WorkloadResourceConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *WorkloadResourceConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.WorkloadMemorySpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *WorkloadResourceConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *WorkloadResourceConfigController) reconcile(ctx context.Context, r controller.Runtime) error {
	r.StartTrackingOutputs()

	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)

	switch {
	case state.IsNotFoundError(err):
	case err != nil:
		return fmt.Errorf("error getting machine config: %w", err)
	case cfg.Config() != nil:
		spec := ctrl.project(cfg.Config())

		if err = safe.WriterModify(ctx, r, runtime.NewWorkloadMemorySpec(), func(res *runtime.WorkloadMemorySpec) error {
			*res.TypedSpec() = spec

			return nil
		}); err != nil {
			return fmt.Errorf("error updating workload memory spec: %w", err)
		}
	}

	return safe.CleanupOutputs[*runtime.WorkloadMemorySpec](ctx, r)
}

func (ctrl *WorkloadResourceConfigController) project(cfg configcfg.Config) runtime.WorkloadMemorySpecSpec {
	if ctrl.V1Alpha1Mode.InContainer() {
		return runtime.WorkloadMemorySpecSpec{}
	}

	resources := cfg.WorkloadResourceConfig()
	if resources == nil {
		return runtime.WorkloadMemorySpecSpec{}
	}

	return runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: resources.TalosContainersMemoryLimit(),
		VirtualMachinesLimit: resources.VirtualMachinesMemoryLimit(),
	}
}
