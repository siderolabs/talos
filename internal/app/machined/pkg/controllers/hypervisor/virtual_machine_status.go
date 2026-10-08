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
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// VirtualMachineStatusController combines desired VMs with independent libvirt observations, and
// with the host links their interfaces need: a spec that cannot be rendered on this host loses
// its power intent.
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
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.ContentLibraryStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.SystemInformationType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.NUMATopologyType,
			ID:        optional.Some(hardware.NUMATopologyID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller.
func (*VirtualMachineStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineStatusType,
			Kind: controller.OutputExclusive,
		},
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

//nolint:gocyclo
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

	linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, runtime)
	if err != nil {
		return fmt.Errorf("list link statuses: %w", err)
	}

	links := newHostLinks(linkStatuses)

	resolvedDisks, err := listResolvedDisks(ctx, runtime)
	if err != nil {
		return err
	}

	topology, err := readNUMATopology(ctx, runtime)
	if err != nil {
		return err
	}

	machineUUID, machineErr := getMachineUUID(ctx, runtime)

	var errs error

	for spec := range specs.All() {
		name := spec.Metadata().ID()

		// VirtualMachineDomainSpecController withdraws the power intent of a spec it cannot
		// render, so an observed domain is on its way out: that obstacle outranks its apparent
		// readiness. Rendering here rather than reading the obstacle off the domain spec keeps
		// the reason legible even before a domain spec exists.
		rendered, renderErr := renderVirtualMachineDomainWithSeed(ctx, runtime, name, spec.TypedSpec(), links, resolvedDisks)

		// Placement is checked on the exact rendered definition, as VirtualMachineController admits
		// it. Unlike a render failure it does not withdraw the power intent: a running domain keeps
		// running, and only its next start or replacement is held back.
		if renderErr == nil && spec.TypedSpec().PowerState == hypervisor.VirtualMachinePowerStateRunning.String() {
			renderErr = validateDomainPlacement(name, rendered.DomainXML, topology)
		}

		status := composeVirtualMachineStatus(spec.TypedSpec().PowerState, name, machineUUID, machineErr, renderErr, byName[name])

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

// renderStage grades a spec that cannot be rendered: a link the host has not brought up yet is
// worth waiting for, anything else needs the config changed.
func renderStage(err error) hypervisor.VirtualMachineStage {
	if errors.Is(err, errLinkNotFound) || errors.Is(err, errDiskNotReady) || errors.Is(err, errPlacementPending) {
		return hypervisor.VirtualMachineStagePending
	}

	return hypervisor.VirtualMachineStageError
}

func composeVirtualMachineStatus(desired, name string, machineUUID uuid.UUID, machineErr, renderErr error,
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
		if renderErr != nil {
			// The domain was never defined, or it has been withdrawn, and this is why.
			status.Stage = renderStage(renderErr)
			status.Error = renderErr.Error()

			return status
		}

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

	if renderErr != nil {
		status.Stage = renderStage(renderErr)
		status.Error = renderErr.Error()

		return status
	}

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
