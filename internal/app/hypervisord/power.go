// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
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

// Reboot implements machine.HypervisorServiceServer.
//
// Unlike Start and Stop this leaves the power state a virtual machine is driven towards alone, and
// acts only on the guest running under it.
func (s *Service) Reboot(ctx context.Context, in *machine.VirtualMachineRebootRequest) (*machine.VirtualMachineRebootResponse, error) {
	identity, err := s.rebootable(ctx, in.GetName())
	if err != nil {
		return nil, err
	}

	client, err := s.domains(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "connect to the QEMU daemon: %v", err)
	}

	defer client.Close()

	s.powerMu.Lock()
	defer s.powerMu.Unlock()

	// Connecting can block while configuration and controller projections change.
	if err = s.assertRunningAndReady(ctx, in.GetName()); err != nil {
		return nil, err
	}

	// Force means the domain goes away; the controller which owns it defines it again from the
	// configuration which is live, which is the same definition it was running.
	//
	// Remove is idempotent where Reboot is not, so a domain which went away between the readiness
	// check and this call is reported as a reboot which happened. What makes a reboot of a machine
	// which is not running fail is rebootable, not the libvirt call it ends in.
	reboot := client.Reboot
	if in.GetForce() {
		reboot = client.Remove
	}

	if err = reboot(identity); err != nil {
		if errors.Is(err, domain.ErrDomainNotRunning) {
			return nil, status.Error(codes.FailedPrecondition, "virtual machine is not running")
		}

		return nil, status.Errorf(codes.Unavailable, "reboot virtual machine: %v", err)
	}

	return &machine.VirtualMachineRebootResponse{}, nil
}

// drive overrides the power state a virtual machine is driven towards for the current boot, and
// for a stop, records the way it is carried out.
//
// The VM spec's producer power state is left alone, and the override is never persisted: a
// reboot drops it, and the virtual machine goes back to its producer's intent. Nothing here
// waits for the machine to reach the state; VirtualMachineStatus reports the outcome.
func (s *Service) drive(ctx context.Context, name string, powerState hypervisorhelpers.PowerState, mode hypervisorhelpers.StopMode) error {
	if name == "" {
		return status.Error(codes.InvalidArgument, "virtual machine name is required")
	}

	if s.power == nil {
		return status.Error(codes.Unimplemented, "virtual machine power management is not available")
	}

	declared, err := s.declaredPowerState(ctx, name)
	if status.Code(err) == codes.NotFound {
		return status.Errorf(codes.FailedPrecondition, "virtual machine %q has no spec", name)
	}

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

// declaredPowerState reads the producer's intent from the VM spec, not the effective domain state.
func (s *Service) declaredPowerState(ctx context.Context, name string) (hypervisorhelpers.PowerState, error) {
	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](ctx, s.power, name)
	if state.IsNotFoundError(err) {
		return 0, status.Errorf(codes.NotFound, "virtual machine %q has no spec", name)
	}

	if err != nil {
		return 0, fmt.Errorf("read virtual machine spec: %w", err)
	}

	if spec.Metadata().Phase() != resource.PhaseRunning {
		return 0, status.Errorf(codes.FailedPrecondition, "virtual machine %q is tearing down", name)
	}

	switch spec.TypedSpec().PowerState {
	case hypervisorhelpers.PowerStateRunning.String():
		return hypervisorhelpers.PowerStateRunning, nil
	case hypervisorhelpers.PowerStateStopped.String():
		return hypervisorhelpers.PowerStateStopped, nil
	case hypervisorhelpers.PowerStateSuspended.String():
		return hypervisorhelpers.PowerStateSuspended, nil
	default:
		return 0, status.Errorf(codes.FailedPrecondition, "virtual machine %q has no recognized power state", name)
	}
}

// rebootable resolves a virtual machine which is ready to be rebooted.
func (s *Service) rebootable(ctx context.Context, name string) (domain.Domain, error) {
	if name == "" {
		return domain.Domain{}, status.Error(codes.InvalidArgument, "virtual machine name is required")
	}

	if s.domains == nil || s.power == nil {
		return domain.Domain{}, status.Error(codes.Unimplemented, "virtual machine power management is not available")
	}

	if err := s.assertRunningAndReady(ctx, name); err != nil {
		return domain.Domain{}, err
	}

	machineUUID, err := readMachineUUID(ctx, s.power)
	if err != nil {
		return domain.Domain{}, err
	}

	return domain.Domain{Name: name, UUID: domain.UUID(machineUUID, name)}, nil
}

// assertNoPendingStop fences reboot while a stop is not projected yet.
func (s *Service) assertNoPendingStop(ctx context.Context, name string) error {
	override, err := safe.StateGetByID[*hypervisor.VirtualMachinePowerOverride](ctx, s.power, name)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("read power override: %w", err)
	}

	declared, err := s.declaredPowerState(ctx, name)
	if status.Code(err) == codes.NotFound {
		return nil // assertRunningAndReady reports an absent spec as not managed.
	}

	if err != nil {
		return err
	}

	if override != nil {
		if override.TypedSpec().PowerState != hypervisorhelpers.PowerStateRunning.String() {
			return status.Error(codes.FailedPrecondition, "virtual machine is not driven towards running")
		}

		return nil
	}

	if declared != hypervisorhelpers.PowerStateRunning {
		return status.Error(codes.FailedPrecondition, "virtual machine is not driven towards running")
	}

	return nil
}

// assertRunningAndReady refuses a virtual machine which is not driven towards running, or which has
// something outstanding against the definition which is.
func (s *Service) assertRunningAndReady(ctx context.Context, name string) error {
	if err := s.assertNoPendingStop(ctx, name); err != nil {
		return err
	}

	generation, err := s.runningIntentGeneration(ctx, name)
	if err != nil {
		return err
	}

	domainSpec, err := s.readyDomain(ctx, name, generation)
	if err != nil {
		return err
	}

	vmStatus, err := safe.StateGetByID[*hypervisor.VirtualMachineStatus](ctx, s.power, name)
	if state.IsNotFoundError(err) {
		return status.Error(codes.FailedPrecondition, "virtual machine has not been observed yet")
	}

	if err != nil {
		return fmt.Errorf("read virtual machine status: %w", err)
	}

	if vmStatus.TypedSpec().ObservedGeneration != hypervisor.VirtualMachineStatusGeneration(generation, domainSpec) {
		return status.Error(codes.FailedPrecondition, "virtual machine status has not observed its current configuration yet")
	}

	// Taking an unready definition down would be taking down a guest which cannot be defined again
	// until whatever is outstanding against it resolves.
	if vmStatus.TypedSpec().Stage != hypervisorhelpers.VirtualMachineStageReady {
		return status.Errorf(codes.FailedPrecondition, "virtual machine is not ready: %s", vmStatus.TypedSpec().Stage)
	}

	return nil
}

func (s *Service) runningIntentGeneration(ctx context.Context, name string) (string, error) {
	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](ctx, s.power, name)
	if state.IsNotFoundError(err) {
		return "", status.Error(codes.NotFound, "virtual machine is not managed")
	}

	if err != nil {
		return "", fmt.Errorf("read virtual machine spec: %w", err)
	}

	override, err := safe.StateGetByID[*hypervisor.VirtualMachinePowerOverride](ctx, s.power, name)
	if err != nil && !state.IsNotFoundError(err) {
		return "", fmt.Errorf("read power override: %w", err)
	}

	if spec.Metadata().Phase() != resource.PhaseRunning {
		return "", status.Error(codes.FailedPrecondition, "virtual machine is not driven towards running")
	}

	power := spec.TypedSpec().PowerState

	if override != nil {
		power = override.TypedSpec().PowerState
	}

	if power != hypervisorhelpers.PowerStateRunning.String() {
		return "", status.Error(codes.FailedPrecondition, "virtual machine is not driven towards running")
	}

	return hypervisor.VirtualMachineEffectiveGeneration(spec, override), nil
}

func (s *Service) readyDomain(ctx context.Context, name, generation string) (*hypervisor.VirtualMachineDomainSpec, error) {
	domainSpec, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](ctx, s.power, name)
	if state.IsNotFoundError(err) {
		return nil, status.Error(codes.FailedPrecondition, "virtual machine domain has not caught up")
	}

	if err != nil {
		return nil, fmt.Errorf("read virtual machine domain spec: %w", err)
	}

	if domainSpec.Metadata().Phase() != resource.PhaseRunning || domainSpec.TypedSpec().PowerState != hypervisorhelpers.PowerStateRunning.String() ||
		domainSpec.TypedSpec().ObservedGeneration != generation || domainSpec.TypedSpec().DomainXML == "" {
		return nil, status.Error(codes.FailedPrecondition, "virtual machine domain has not caught up")
	}

	return domainSpec, nil
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
