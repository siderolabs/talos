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

// CPUPartitionConfigController projects CPUPartitionConfig into the canonical CPUPartitionSpec.
//
// The spec is published as soon as the machine configuration is active, so consumers can tell
// "projection pending" (no resource) from "no policy" (Enabled=false). Container mode is the
// single gate: the document is still validated, but the projected policy is disabled, so no
// runtime consumer ever acts on it there.
type CPUPartitionConfigController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *CPUPartitionConfigController) Name() string {
	return "runtime.CPUPartitionConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CPUPartitionConfigController) Inputs() []controller.Input {
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
func (ctrl *CPUPartitionConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.CPUPartitionSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CPUPartitionConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

func (ctrl *CPUPartitionConfigController) reconcile(ctx context.Context, r controller.Runtime) error {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting machine config: %w", err)
	}

	r.StartTrackingOutputs()

	if cfg != nil && cfg.Config() != nil {
		var partition configcfg.CPUPartitionConfig

		if !ctrl.V1Alpha1Mode.InContainer() {
			partition = cfg.Config().CPUPartitionConfig()
		}

		if err = safe.WriterModify(ctx, r, runtime.NewCPUPartitionSpec(), func(res *runtime.CPUPartitionSpec) error {
			*res.TypedSpec() = projectCPUPartitionSpec(partition)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating CPU partition spec: %w", err)
		}
	}

	return safe.CleanupOutputs[*runtime.CPUPartitionSpec](ctx, r)
}

// projectCPUPartitionSpec builds the canonical spec; a nil document is a confirmed absence of policy.
func projectCPUPartitionSpec(partition configcfg.CPUPartitionConfig) runtime.CPUPartitionSpecSpec {
	if partition == nil {
		return runtime.CPUPartitionSpecSpec{}
	}

	spec := runtime.CPUPartitionSpecSpec{
		Enabled: true,
		Roots:   map[string]string{},
	}

	for root, cpus := range partition.Roots() {
		spec.Roots[string(root)] = cpus
	}

	for _, slice := range partition.Slices() {
		spec.Slices = append(spec.Slices, runtime.CPUPartitionSliceSpec{
			Name:      slice.Name(),
			CPUs:      slice.CPUs(),
			Exclusive: slice.Exclusive(),
		})
	}

	return spec
}
