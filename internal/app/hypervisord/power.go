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
	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// Start implements machine.HypervisorServiceServer.
func (s *Service) Start(ctx context.Context, in *machine.VirtualMachineStartRequest) (*machine.VirtualMachineStartResponse, error) {
	name := in.GetName()

	s.powerMu.Lock()
	defer s.powerMu.Unlock()

	if err := s.drive(ctx, name, hypervisorhelpers.PowerStateRunning); err != nil {
		return nil, err
	}

	// A start ends whatever stop was asked for before it, finished or not. Left in place, the record
	// of a graceful stop the guest never obeyed would decide how some later stop is carried out.
	if err := s.clearStopMode(ctx, name); err != nil {
		return nil, err
	}

	return &machine.VirtualMachineStartResponse{}, nil
}

// Stop implements machine.HypervisorServiceServer.
func (s *Service) Stop(ctx context.Context, in *machine.VirtualMachineStopRequest) (*machine.VirtualMachineStopResponse, error) {
	name := in.GetName()

	mode := hypervisorhelpers.StopModeGraceful
	if in.GetForce() {
		mode = hypervisorhelpers.StopModeForced
	}

	s.powerMu.Lock()
	defer s.powerMu.Unlock()

	// Recorded before the machine is driven towards stopped, so that the controller which carries
	// the stop out cannot see the power state without the way it was asked for.
	previous, err := s.setStopMode(ctx, name, mode)
	if err != nil {
		return nil, err
	}

	if err = s.drive(ctx, name, hypervisorhelpers.PowerStateStopped); err != nil {
		// This stop never started, so its record describes nothing: put back whatever was recorded
		// before it, which may be a stop still in flight. VirtualMachineStopModeController keeps the
		// records of machines driven towards running, so it would not retire this one.
		if restoreErr := s.restoreStopMode(ctx, name, previous); restoreErr != nil {
			return nil, errors.Join(err, restoreErr)
		}

		return nil, err
	}

	return &machine.VirtualMachineStopResponse{}, nil
}

// setStopMode records how the next stop of a virtual machine is to be carried out.
//
// The record is stamped with the owner of the controller which retires it, which is the only writer
// allowed to destroy it. Absence of a record is absence of a stop, not a forced one: a stop driven
// by a machine configuration patch rather than by this API carries no record and destroys the
// domain, which is what every stop did before this existed.
//
// It returns the mode recorded before, or an empty string when there was none.
func (s *Service) setStopMode(ctx context.Context, name string, mode hypervisorhelpers.StopMode) (string, error) {
	if name == "" {
		return "", status.Error(codes.InvalidArgument, "virtual machine name is required")
	}

	if s.power == nil {
		return "", status.Error(codes.Unimplemented, "virtual machine power management is not available")
	}

	var previous string

	stopMode := hypervisor.NewVirtualMachineStopMode(hypervisor.NamespaceName, name)
	stopMode.TypedSpec().Mode = mode.String()

	err := s.power.Create(ctx, stopMode, state.WithCreateOwner(hypervisor.VirtualMachineStopModeOwner))
	if state.IsConflictError(err) {
		_, err = safe.StateUpdateWithConflicts(ctx, s.power, stopMode.Metadata(),
			func(res *hypervisor.VirtualMachineStopMode) error {
				previous = res.TypedSpec().Mode
				res.TypedSpec().Mode = mode.String()

				return nil
			},
			state.WithUpdateOwner(hypervisor.VirtualMachineStopModeOwner),
		)
	}

	if err != nil {
		return "", fmt.Errorf("record the stop mode of virtual machine %q: %w", name, err)
	}

	return previous, nil
}

// restoreStopMode puts back the mode recorded before a stop which did not start, withdrawing the
// record when there was none.
func (s *Service) restoreStopMode(ctx context.Context, name, previous string) error {
	if previous == "" {
		return s.clearStopMode(ctx, name)
	}

	mode, err := hypervisorhelpers.StopModeString(previous)
	if err != nil {
		return fmt.Errorf("restore the stop mode of virtual machine %q: %w", name, err)
	}

	_, err = s.setStopMode(ctx, name, mode)

	return err
}

// clearStopMode withdraws the record of how the next stop of a virtual machine is to be carried out.
func (s *Service) clearStopMode(ctx context.Context, name string) error {
	if s.power == nil {
		return status.Error(codes.Unimplemented, "virtual machine power management is not available")
	}

	err := s.power.Destroy(ctx,
		hypervisor.NewVirtualMachineStopMode(hypervisor.NamespaceName, name).Metadata(),
		state.WithDestroyOwner(hypervisor.VirtualMachineStopModeOwner),
	)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("withdraw the stop mode of virtual machine %q: %w", name, err)
	}

	return nil
}

// Reboot implements machine.HypervisorServiceServer.
//
// Unlike Start and Stop this leaves the machine configuration alone: the power state a virtual
// machine is driven towards does not change, only the guest running under it.
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

// drive writes the power state a virtual machine is driven towards into the machine configuration.
//
// Nothing here waits for the machine to reach that state: the configuration is what is being set,
// and VirtualMachineStatus is what reports the outcome.
func (s *Service) drive(ctx context.Context, name string, powerState hypervisorhelpers.PowerState) error {
	if name == "" {
		return status.Error(codes.InvalidArgument, "virtual machine name is required")
	}

	if s.config == nil {
		return status.Error(codes.Unimplemented, "virtual machine power management is not available")
	}

	return s.config.PatchConfiguration(ctx, func(current coreconfig.Container) (coreconfig.Provider, error) {
		documents := current.Documents()
		patched := make([]config.Document, 0, len(documents))
		found := false

		for _, document := range documents {
			vm, ok := document.(*hypervisorcfg.VirtualMachineConfigV1Alpha1)
			if !ok || vm.Name() != name {
				patched = append(patched, document)

				continue
			}

			// Documents of the live container are not ours to modify.
			clone, ok := vm.Clone().(*hypervisorcfg.VirtualMachineConfigV1Alpha1)
			if !ok {
				return nil, fmt.Errorf("cloned document of virtual machine %q has an unexpected type", name)
			}

			clone.PowerStateConfig = powerState

			patched = append(patched, clone)
			found = true
		}

		if !found {
			return nil, status.Errorf(codes.FailedPrecondition,
				"virtual machine %q is not declared in the machine configuration", name)
		}

		return container.New(patched...)
	})
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

// assertRunningAndReady refuses a virtual machine which is not driven towards running, or which has
// something outstanding against the definition which is.
func (s *Service) assertRunningAndReady(ctx context.Context, name string) error {
	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](ctx, s.power, name)
	if state.IsNotFoundError(err) {
		return status.Error(codes.NotFound, "virtual machine is not managed")
	}

	if err != nil {
		return fmt.Errorf("read virtual machine spec: %w", err)
	}

	if spec.Metadata().Phase() != resource.PhaseRunning ||
		spec.TypedSpec().PowerState != hypervisorhelpers.PowerStateRunning.String() {
		return status.Error(codes.FailedPrecondition, "virtual machine is not driven towards running")
	}

	vmStatus, err := safe.StateGetByID[*hypervisor.VirtualMachineStatus](ctx, s.power, name)
	if state.IsNotFoundError(err) {
		return status.Error(codes.FailedPrecondition, "virtual machine has not been observed yet")
	}

	if err != nil {
		return fmt.Errorf("read virtual machine status: %w", err)
	}

	// Taking an unready definition down would be taking down a guest which cannot be defined again
	// until whatever is outstanding against it resolves.
	if vmStatus.TypedSpec().Stage != hypervisor.VirtualMachineStageReady {
		return status.Errorf(codes.FailedPrecondition, "virtual machine is not ready: %s", vmStatus.TypedSpec().Stage)
	}

	return nil
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
