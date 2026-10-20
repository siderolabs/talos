// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	vmprofile "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/profile"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// VirtualMachineProfileSpecController publishes builtin and configured guest hardware profiles.
type VirtualMachineProfileSpecController struct{}

// Name implements controller.Controller.
func (*VirtualMachineProfileSpecController) Name() string {
	return "hypervisor.VirtualMachineProfileSpecController"
}

// Inputs implements controller.Controller.
func (*VirtualMachineProfileSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller.
func (*VirtualMachineProfileSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineProfileSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller.
func (ctrl *VirtualMachineProfileSpecController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineProfileSpecController) reconcile(ctx context.Context, runtime controller.Runtime) error {
	machine, err := safe.ReaderGetByID[*config.MachineConfig](ctx, runtime, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("get machine config: %w", err)
	}

	runtime.StartTrackingOutputs()

	var errs error

	for _, builtin := range vmprofile.Profiles() {
		if err := safe.WriterModify(ctx, runtime,
			builtin,
			func(res *hypervisor.VirtualMachineProfileSpec) error {
				*res.TypedSpec() = *builtin.TypedSpec()

				return nil
			},
		); err != nil {
			errs = errors.Join(errs, fmt.Errorf("write builtin profile %q: %w", builtin.Metadata().ID(), err))
		}
	}

	if machine != nil && machine.Config() != nil {
		for _, profile := range machine.Config().VirtualMachineProfileConfigs() {
			projected, projectionErr := projectVirtualMachineProfile(profile)
			if projectionErr != nil {
				errs = errors.Join(errs, fmt.Errorf("profile %q: %w", profile.Name(), projectionErr))

				continue
			}

			if err := safe.WriterModify(ctx, runtime,
				hypervisor.NewVirtualMachineProfileSpec(hypervisor.NamespaceName, profile.Name()),
				func(res *hypervisor.VirtualMachineProfileSpec) error {
					*res.TypedSpec() = projected

					return nil
				},
			); err != nil {
				errs = errors.Join(errs, fmt.Errorf("write profile %q: %w", profile.Name(), err))
			}
		}
	}

	return errors.Join(errs, safe.CleanupOutputs[*hypervisor.VirtualMachineProfileSpec](ctx, runtime))
}

func projectVirtualMachineProfile(profile configcfg.VirtualMachineProfileConfig) (hypervisor.VirtualMachineProfileSpecSpec, error) {
	cpu := profile.CPU()

	result := hypervisor.VirtualMachineProfileSpecSpec{
		Machine: profile.Machine(),
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode:     cpu.Mode(),
			Model:    cpu.Model(),
			Fallback: cpu.Fallback(),
		},
	}
	for _, feature := range cpu.Features() {
		result.CPU.Features = append(result.CPU.Features, hypervisor.VirtualMachineProfileCPUFeatureSpec{
			Name:   feature.Name(),
			Policy: feature.Policy(),
		})
	}

	return result, vmprofile.ValidateCustom(profile.Name(), result)
}
