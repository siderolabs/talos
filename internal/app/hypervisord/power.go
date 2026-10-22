// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// Start implements machine.HypervisorServiceServer.
func (s *Service) Start(ctx context.Context, in *machine.VirtualMachineStartRequest) (*machine.VirtualMachineStartResponse, error) {
	name := in.GetName()

	s.powerMu.Lock()
	defer s.powerMu.Unlock()

	if err := s.drive(ctx, name, hypervisorhelpers.PowerStateRunning, hypervisorhelpers.StopModeUnknown); err != nil {
		return nil, err
	}

	return &machine.VirtualMachineStartResponse{}, nil
}

// Stop implements machine.HypervisorServiceServer.
func (s *Service) Stop(ctx context.Context, in *machine.VirtualMachineStopRequest) (*machine.VirtualMachineStopResponse, error) {
	s.powerMu.Lock()
	defer s.powerMu.Unlock()

	mode := hypervisorhelpers.StopModeGraceful
	if in.GetForce() {
		mode = hypervisorhelpers.StopModeForced
	}

	if err := s.drive(ctx, in.GetName(), hypervisorhelpers.PowerStateStopped, mode); err != nil {
		return nil, err
	}

	return &machine.VirtualMachineStopResponse{}, nil
}

// drive overrides the power state a virtual machine is driven towards for the current boot, and
// for a stop, records the way it is carried out.
//
// The machine configuration is left alone, and the override is never persisted: a reboot drops it,
// and the virtual machine goes back to the power state its configuration declares. Nothing here
// waits for the machine to reach the state; VirtualMachineStatus reports the outcome.
func (s *Service) drive(ctx context.Context, name string, powerState hypervisorhelpers.PowerState, mode hypervisorhelpers.StopMode) error {
	if name == "" {
		return status.Error(codes.InvalidArgument, "virtual machine name is required")
	}

	if s.power == nil {
		return status.Error(codes.Unimplemented, "virtual machine power management is not available")
	}

	declared, err := s.declaredPowerState(ctx, name)
	if err != nil {
		return err
	}

	override := hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, name)

	// Starting a machine configured to run leaves nothing to override.
	if powerState == hypervisorhelpers.PowerStateRunning && declared == hypervisorhelpers.PowerStateRunning {
		if err = s.power.Destroy(ctx, override.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("remove power override of virtual machine %q: %w", name, err)
		}

		return nil
	}

	return s.writePowerOverride(ctx, override, powerState, mode)
}

func (s *Service) writePowerOverride(ctx context.Context, override *hypervisor.VirtualMachinePowerOverride,
	powerState hypervisorhelpers.PowerState, mode hypervisorhelpers.StopMode,
) error {
	name := override.Metadata().ID()

	current, err := safe.StateGetByID[*hypervisor.VirtualMachinePowerOverride](ctx, s.power, name)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("read power override of virtual machine %q: %w", name, err)
	}

	// Asking again for the stop in effect keeps its request, so the guest is not asked anew.
	if current != nil && current.TypedSpec().PowerState == powerState.String() &&
		hypervisorhelpers.VirtualMachineStopRequestMode(current.TypedSpec().StopRequest) == mode {
		return nil
	}

	spec := hypervisor.VirtualMachinePowerOverrideSpec{PowerState: powerState.String()}

	if mode != hypervisorhelpers.StopModeUnknown {
		// A fresh ID per stop: the controller presses the power button once per request it sees.
		spec.StopRequest = hypervisorhelpers.VirtualMachineStopRequest(mode, uuid.NewString())
	}

	if err = safe.StateModify(ctx, s.power, override, func(res *hypervisor.VirtualMachinePowerOverride) error {
		*res.TypedSpec() = spec

		return nil
	}); err != nil {
		return fmt.Errorf("write power override of virtual machine %q: %w", name, err)
	}

	return nil
}

// declaredPowerState reads the power state the machine configuration declares for a virtual machine.
func (s *Service) declaredPowerState(ctx context.Context, name string) (hypervisorhelpers.PowerState, error) {
	active, err := safe.StateGetByID[*configres.MachineConfig](ctx, s.power, configres.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, fmt.Errorf("read active machine configuration: %w", err)
	}

	if active != nil && active.Config() != nil {
		for _, vm := range active.Config().VirtualMachineConfigs() {
			if vm.Name() == name {
				return vm.PowerState(), nil
			}
		}
	}

	return 0, status.Errorf(codes.FailedPrecondition, "virtual machine %q is not declared in the machine configuration", name)
}

// readMachineUUID reads the identity every domain of this host is named under.
func readMachineUUID(ctx context.Context, resources state.State) (uuid.UUID, error) {
	system, err := safe.StateGetByID[*hardware.SystemInformation](ctx, resources, hardware.SystemInformationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("read system information: %w", err)
	}

	machineUUID, err := domain.ParseMachineUUID(system.TypedSpec().UUID)
	if err != nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	return machineUUID, nil
}
