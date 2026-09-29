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
	"github.com/google/uuid"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// VirtualMachineStatusController combines desired VMs with independent libvirt observations.
type VirtualMachineStatusController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller.
func (*VirtualMachineStatusController) Name() string {
	return "hypervisor.VirtualMachineStatusController"
}

// Inputs implements controller.Controller.
func (*VirtualMachineStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{Namespace: hypervisor.NamespaceName, Type: hypervisor.VirtualMachineSpecType, Kind: controller.InputWeak},
		{Namespace: hypervisor.NamespaceName, Type: hypervisor.VirtualMachineDomainStatusType, Kind: controller.InputWeak},
		{Namespace: hardware.NamespaceName, Type: hardware.SystemInformationType, Kind: controller.InputWeak},
	}
}

// Outputs implements controller.Controller.
func (*VirtualMachineStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{Type: hypervisor.VirtualMachineStatusType, Kind: controller.OutputExclusive},
	}
}

// Run implements controller.Controller.
func (ctrl *VirtualMachineStatusController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

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

func (ctrl *VirtualMachineStatusController) reconcile(ctx context.Context, runtime controller.Runtime) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, runtime)
	if err != nil {
		return fmt.Errorf("list virtual machine specs: %w", err)
	}

	runtime.StartTrackingOutputs()

	if specs.Len() == 0 {
		return safe.CleanupOutputs[*hypervisor.VirtualMachineStatus](ctx, runtime)
	}

	domains, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainStatus](ctx, runtime)
	if err != nil {
		return fmt.Errorf("list virtual machine domain statuses: %w", err)
	}

	byName := make(map[string]*hypervisor.VirtualMachineDomainStatus, domains.Len())
	for domain := range domains.All() {
		byName[domain.Metadata().ID()] = domain
	}

	machineUUID, machineErr := getMachineUUID(ctx, runtime)

	var errs error

	for spec := range specs.All() {
		name := spec.Metadata().ID()
		status := composeVirtualMachineStatus(spec.TypedSpec().PowerState, name, machineUUID, machineErr, byName[name])

		if writeErr := safe.WriterModify(ctx, runtime,
			hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, name),
			func(res *hypervisor.VirtualMachineStatus) error {
				*res.TypedSpec() = status

				return nil
			},
		); writeErr != nil {
			errs = errors.Join(errs, fmt.Errorf("write virtual machine status %q: %w", name, writeErr))
		}
	}

	return errors.Join(errs, safe.CleanupOutputs[*hypervisor.VirtualMachineStatus](ctx, runtime))
}

func composeVirtualMachineStatus(desired, name string, machineUUID uuid.UUID, machineErr error,
	domain *hypervisor.VirtualMachineDomainStatus,
) hypervisor.VirtualMachineStatusSpec {
	status := hypervisor.VirtualMachineStatusSpec{PowerState: hypervisor.VirtualMachinePowerStateUnknown}

	if desired != hypervisor.VirtualMachinePowerStateRunning.String() &&
		desired != hypervisor.VirtualMachinePowerStateStopped.String() {
		status.Stage = hypervisor.VirtualMachineStageError
		status.Error = fmt.Sprintf("unsupported power state %q", desired)

		return status
	}

	switch {
	case machineErr != nil:
		status.Error = machineErr.Error()

		return status
	case domain == nil:
		status.Error = "domain has not been observed"

		return status
	}

	if domain.TypedSpec().UUID != libvirtdomain.UUID(machineUUID, name).String() {
		status.Stage = hypervisor.VirtualMachineStageError
		status.Error = "domain name is occupied by another VM"

		return status
	}

	status.PowerState = domain.TypedSpec().PowerState
	if domain.TypedSpec().Error != "" {
		status.Stage = hypervisor.VirtualMachineStageError
		status.Error = domain.TypedSpec().Error

		return status
	}

	status.Stage = hypervisor.VirtualMachineStagePending

	return reconcileVirtualMachinePower(desired, status)
}

func reconcileVirtualMachinePower(desired string,
	status hypervisor.VirtualMachineStatusSpec,
) hypervisor.VirtualMachineStatusSpec {
	switch {
	case desired == hypervisor.VirtualMachinePowerStateStopped.String():
		status.Error = "domain is still defined"
	default:
		if status.PowerState.String() == desired {
			status.Stage = hypervisor.VirtualMachineStageReady
		}
	}

	return status
}
