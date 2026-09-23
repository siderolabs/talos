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

	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// VirtualMachineSpecController projects machine configuration into backend-neutral specs.
//
// Specs are shared outputs: alternate producers may author other VM IDs. COSI
// rejects changes to same-ID specs owned by other producers (including unowned
// specs). An identical no-op succeeds without adopting the external resource.
// Output tracking cleans up only this controller's specs. Producers must not
// use this controller's owner identity.
type VirtualMachineSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineSpecController) Name() string {
	return "hypervisor.VirtualMachineSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VirtualMachineSpecController) Inputs() []controller.Input {
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
func (ctrl *VirtualMachineSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineSpecController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
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

func (ctrl *VirtualMachineSpecController) reconcile(ctx context.Context, runtime controller.Runtime) error {
	machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, runtime, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get machine config: %w", err)
	}

	runtime.StartTrackingOutputs()

	if machineConfig == nil || machineConfig.Config() == nil {
		return safe.CleanupOutputs[*hypervisor.VirtualMachineSpec](ctx, runtime)
	}

	var errs []error

	for _, vm := range machineConfig.Config().VirtualMachineConfigs() {
		if err := safe.WriterModify(ctx, runtime,
			hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vm.Name()),
			func(res *hypervisor.VirtualMachineSpec) error {
				*res.TypedSpec() = projectVirtualMachineSpec(vm)

				return nil
			},
		); err != nil {
			errs = append(errs, fmt.Errorf("failed to write virtual machine spec %q: %w", vm.Name(), err))
		}
	}

	return errors.Join(append(errs, safe.CleanupOutputs[*hypervisor.VirtualMachineSpec](ctx, runtime))...)
}

func projectVirtualMachineSpec(vm configcfg.VirtualMachineConfig) hypervisor.VirtualMachineSpecSpec {
	spec := hypervisor.VirtualMachineSpecSpec{
		CPU: hypervisor.VirtualMachineCPUSpec{
			Count: vm.CPU().Count(),
		},
		Memory: hypervisor.VirtualMachineMemorySpec{
			Size: vm.Memory().Size(),
			Ballooning: hypervisor.VirtualMachineMemoryBallooningSpec{
				Enabled: vm.Memory().Ballooning().Enabled(),
			},
		},
		PowerState: vm.PowerState().String(),
		Firmware: hypervisor.VirtualMachineFirmwareSpec{
			Type:       vm.Firmware().Type().String(),
			SecureBoot: vm.Firmware().SecureBoot().Enabled(),
		},
		Console: hypervisor.VirtualMachineConsoleSpec{
			Serial: vm.Console().Serial().Enabled(),
			VNC:    vm.Console().VNC().Enabled(),
		},
	}

	for _, disk := range vm.Disks() {
		intent := hypervisor.VirtualMachineDiskSpec{
			Name:      disk.Name(),
			Pool:      disk.Pool(),
			Size:      disk.Size(),
			Format:    disk.Format().String(),
			Bus:       disk.Bus().String(),
			Type:      disk.Type().String(),
			BootOrder: disk.BootOrder(),
			Provision: hypervisor.VirtualMachineDiskProvisionSpec{
				Blank: disk.Provision().Blank(),
			},
		}
		if image, ok := disk.Provision().FromImage().Get(); ok {
			intent.Provision.FromImage = &hypervisor.VirtualMachineDiskFromImageSpec{
				Library: image.Library(),
				File:    image.File(),
				Digest:  image.Digest(),
				Mode:    image.Mode().String(),
			}
		}

		spec.Disks = append(spec.Disks, intent)
	}

	return spec
}
