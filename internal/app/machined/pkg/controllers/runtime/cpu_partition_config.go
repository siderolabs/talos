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
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// CPUPartitionConfigController captures the CPU partition policy of the boot into CPUPartitionSpec.
//
// The spec is published from the first active machine configuration, including when it declares no
// policy, and never changes afterwards: a different policy takes effect after a reboot. In container
// mode the policy is always disabled.
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

		captured, err := ctrl.capture(ctx, r)
		if err != nil || captured {
			return err
		}
	}
}

// capture publishes the spec from the active configuration, reporting whether the spec exists. A
// controller restart finds the spec it already captured and leaves it alone.
func (ctrl *CPUPartitionConfigController) capture(ctx context.Context, r controller.Runtime) (bool, error) {
	if _, err := safe.ReaderGetByID[*runtime.CPUPartitionSpec](ctx, r, runtime.CPUPartitionSpecID); err == nil {
		return true, nil
	} else if !state.IsNotFoundError(err) {
		return false, fmt.Errorf("error getting CPU partition spec: %w", err)
	}

	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("error getting machine config: %w", err)
	}

	if cfg.Config() == nil {
		return false, nil
	}

	spec := runtime.CPUPartitionSpecSpec{}

	if !ctrl.V1Alpha1Mode.InContainer() {
		spec = container.ProjectCPUPartition(cfg.Config().CPUPartitionConfig())
	}

	if err = safe.WriterModify(ctx, r, runtime.NewCPUPartitionSpec(), func(res *runtime.CPUPartitionSpec) error {
		*res.TypedSpec() = spec

		return nil
	}); err != nil {
		return false, fmt.Errorf("error publishing CPU partition spec: %w", err)
	}

	return true, nil
}
