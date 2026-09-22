// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	hardwarepb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/hardware"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// CPUScalingConfigController resolves CPUScalingConfig documents against the cpufreq policies
// discovered on the machine, producing a CPUScalingSpec per matched policy.
type CPUScalingConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *CPUScalingConfigController) Name() string {
	return "hardware.CPUScalingConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CPUScalingConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		// selectors match against the discovered policies, so a hotplug re-resolves the configuration
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.CPUScalingStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CPUScalingConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.CPUScalingSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CPUScalingConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r); err != nil {
			return err
		}
	}
}

func (ctrl *CPUScalingConfigController) reconcile(ctx context.Context, r controller.Runtime) error {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting machine config: %w", err)
	}

	var scalingConfigs []configconfig.CPUScalingConfig

	if cfg != nil {
		scalingConfigs = cfg.Config().CPUScalingConfigs()
	}

	// "first match wins" is by document name: the order after config merging is not deterministic
	slices.SortFunc(scalingConfigs, func(a, b configconfig.CPUScalingConfig) int {
		return strings.Compare(a.Name(), b.Name())
	})

	statuses, err := safe.ReaderListAll[*hardware.CPUScalingStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing CPUScalingStatus resources: %w", err)
	}

	r.StartTrackingOutputs()

	for status := range statuses.All() {
		id := status.Metadata().ID()

		var statusSpec hardwarepb.CPUScalingStatusSpec

		if err = proto.ResourceSpecToProto(status, &statusSpec); err != nil {
			return fmt.Errorf("error converting CPUScalingStatus %q to proto: %w", id, err)
		}

		// Selectors match the hardware description only. A selector reading back a value this
		// controller sets would oscillate: `cpu.governor == "schedutil"` with `governor: performance`
		// matches, applies, stops matching, gets restored, and matches again forever.
		statusSpec.Governor = ""
		statusSpec.EnergyPerformancePreference = ""

		matched, err := ctrl.match(scalingConfigs, &statusSpec)
		if err != nil {
			return fmt.Errorf("error matching cpufreq policy %q: %w", id, err)
		}

		if matched == nil {
			continue
		}

		if err = safe.WriterModify(ctx, r, hardware.NewCPUScalingSpec(id), func(res *hardware.CPUScalingSpec) error {
			*res.TypedSpec() = hardware.CPUScalingSpecSpec{
				Governor:                    matched.Governor(),
				EnergyPerformancePreference: matched.EnergyPerformancePreference(),
				MinFrequencyKhz:             matched.MinFrequencyKhz(),
				MaxFrequencyKhz:             matched.MaxFrequencyKhz(),
				ConfigName:                  matched.Name(),
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating CPUScalingSpec resource %q: %w", id, err)
		}
	}

	return safe.CleanupOutputs[*hardware.CPUScalingSpec](ctx, r)
}

// match returns the first config whose selector claims the policy, or nil when none does.
func (ctrl *CPUScalingConfigController) match(
	scalingConfigs []configconfig.CPUScalingConfig, statusSpec *hardwarepb.CPUScalingStatusSpec,
) (configconfig.CPUScalingConfig, error) {
	for _, scalingConfig := range scalingConfigs {
		matches, err := scalingConfig.CPUScalingSelector().EvalBool(celenv.CPUScalingLocator(), map[string]any{
			"cpu": statusSpec,
		})
		if err != nil {
			return nil, fmt.Errorf("error evaluating selector of %q: %w", scalingConfig.Name(), err)
		}

		if matches {
			return scalingConfig, nil
		}
	}

	return nil, nil //nolint:nilnil
}
