// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	vmprofile "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/profile"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
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
			Type:      hypervisor.VirtualMachinePowerOverrideType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineProfileSpecType,
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
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainSpecType,
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

	overrides, err := safe.ReaderListAll[*hypervisor.VirtualMachinePowerOverride](ctx, runtime)
	if err != nil {
		return fmt.Errorf("list virtual machine power overrides: %w", err)
	}

	byOverride := make(map[string]*hypervisor.VirtualMachinePowerOverride, overrides.Len())
	for override := range overrides.All() {
		byOverride[override.Metadata().ID()] = override
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

	// The stop mode reported is the one the domain spec carries, not the one recorded: a stop the
	// controller demands of a definition it cannot render is forced, whatever was asked for.
	definitions, err := listVirtualMachineDomainSpecs(ctx, runtime)
	if err != nil {
		return err
	}

	var errs error

	for spec := range specs.All() {
		name := spec.Metadata().ID()

		effective := spec.TypedSpec().DeepCopy()

		if override := byOverride[name]; override != nil {
			effective.PowerState = override.TypedSpec().PowerState
		}

		generation := hypervisor.VirtualMachineEffectiveGeneration(spec, byOverride[name])

		// VirtualMachineDomainSpecController withdraws the power intent of a spec it cannot
		// render, so an observed domain is on its way out: that obstacle outranks its apparent
		// readiness. Rendering here rather than reading the obstacle off the domain spec keeps
		// the reason legible even before a domain spec exists.
		domainXML, attachedDisks, seedID, renderErr := renderVirtualMachineDomainWithSeed(ctx, runtime, name, spec.TypedSpec(), links, resolvedDisks)

		// Placement is checked on the exact rendered definition, as VirtualMachineController admits
		// it. Unlike a render failure it does not withdraw the power intent: a running domain keeps
		// running, and only its next start or replacement is held back.
		if renderErr == nil && effective.PowerState == hypervisorhelpers.VirtualMachinePowerStateRunning.String() {
			renderErr = validateDomainPlacement(name, domainXML, topology)
		}

		definition := definitions[name]
		stopMode := ""
		released := false

		if definition != nil {
			stopMode = definition.TypedSpec().StopMode
			released = definition.TypedSpec().PowerState == hypervisorhelpers.PowerStateStopped.String() &&
				!definition.Metadata().Finalizers().Has((&VirtualMachineController{}).Name())
		}

		status := composeVirtualMachineStatus(&effective, stopMode, name, released, machineUUID, machineErr, renderErr, byName[name])
		setVirtualMachineStatusGeneration(&status, &effective, generation, definition, domainXML, attachedDisks, seedID)

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

func setVirtualMachineStatusGeneration(status *hypervisor.VirtualMachineStatusSpec, effective *hypervisor.VirtualMachineSpecSpec,
	generation string, definition *hypervisor.VirtualMachineDomainSpec, domainXML string, attachedDisks []string, seedID string,
) {
	status.ObservedGeneration = generation

	if status.Stage == hypervisorhelpers.VirtualMachineStageReady && effective.PowerState == hypervisorhelpers.VirtualMachinePowerStateRunning.String() &&
		(definition == nil || definition.TypedSpec().ObservedGeneration != generation ||
			!matchesVMStartIntent(definition.TypedSpec(), domainXML, attachedDisks, seedID, effective.PowerState)) {
		status.Stage = hypervisorhelpers.VirtualMachineStagePending
		status.Error = "domain definition has not caught up"
	}

	if definition != nil && definition.TypedSpec().ObservedGeneration == generation {
		status.ObservedGeneration = hypervisor.VirtualMachineStatusGeneration(generation, definition)
	}
}

func listVirtualMachineDomainSpecs(ctx context.Context, reader controller.Reader) (map[string]*hypervisor.VirtualMachineDomainSpec, error) {
	domainSpecs, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainSpec](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("list virtual machine domain specs: %w", err)
	}

	definitions := make(map[string]*hypervisor.VirtualMachineDomainSpec, domainSpecs.Len())

	for domainSpec := range domainSpecs.All() {
		definitions[domainSpec.Metadata().ID()] = domainSpec
	}

	return definitions, nil
}

// renderStage grades a spec that cannot be rendered: a link the host has not brought up yet is
// worth waiting for, anything else needs the config changed.
func renderStage(err error) hypervisorhelpers.VirtualMachineStage {
	if errors.Is(err, errLinkNotFound) || errors.Is(err, errDiskNotReady) || errors.Is(err, errPlacementPending) || errors.Is(err, vmprofile.ErrNotReady) {
		return hypervisorhelpers.VirtualMachineStagePending
	}

	return hypervisorhelpers.VirtualMachineStageError
}

// released reports that the domain spec is stopped and VirtualMachineController holds no claim on
// it: that controller lets go only once its own libvirt lookup finds the domain gone, so an absent
// observation is then a finished stop rather than a scan which is failing.
func composeVirtualMachineStatus(spec *hypervisor.VirtualMachineSpecSpec, stopMode, name string, released bool, machineUUID uuid.UUID,
	machineErr, renderErr error, domain *hypervisor.VirtualMachineDomainStatus,
) hypervisor.VirtualMachineStatusSpec {
	desired := spec.PowerState
	status := hypervisor.VirtualMachineStatusSpec{PowerState: hypervisorhelpers.VirtualMachinePowerStateUnknown}

	if desired != hypervisorhelpers.VirtualMachinePowerStateRunning.String() &&
		desired != hypervisorhelpers.VirtualMachinePowerStateStopped.String() {
		status.Stage = hypervisorhelpers.VirtualMachineStageError
		status.Error = fmt.Sprintf("unsupported power state %q", desired)

		return status
	}

	switch {
	case machineErr != nil:
		status.Error = machineErr.Error()

		return status
	case domain == nil:
		return composeUnobservedVirtualMachineStatus(desired, released, renderErr)
	}

	if domain.TypedSpec().UUID != libvirtdomain.UUID(machineUUID, name).String() {
		status.Stage = hypervisorhelpers.VirtualMachineStageError
		status.Error = "domain name is occupied by another VM"

		return status
	}

	status.PowerState = domain.TypedSpec().PowerState
	status.Interfaces = domain.TypedSpec().Interfaces
	status.Addresses = flattenGuestAddresses(domain.TypedSpec().Interfaces)

	if domain.TypedSpec().Error != "" {
		status.Stage = hypervisorhelpers.VirtualMachineStageError
		status.Error = domain.TypedSpec().Error

		return status
	}

	status.Stage = hypervisorhelpers.VirtualMachineStagePending

	if renderErr != nil {
		status.Stage = renderStage(renderErr)
		status.Error = renderErr.Error()

		return status
	}

	return reconcileVirtualMachinePower(spec.PowerState, stopMode, status)
}

func composeUnobservedVirtualMachineStatus(desired string, released bool, renderErr error) hypervisor.VirtualMachineStatusSpec {
	status := hypervisor.VirtualMachineStatusSpec{PowerState: hypervisorhelpers.VirtualMachinePowerStateUnknown}

	if renderErr != nil {
		// The domain was never defined, or it has been withdrawn, and this is why.
		status.Stage = renderStage(renderErr)
		status.Error = renderErr.Error()

		return status
	}

	if desired == hypervisorhelpers.VirtualMachinePowerStateStopped.String() && released {
		status.PowerState = hypervisorhelpers.VirtualMachinePowerStateStopped
		status.Stage = hypervisorhelpers.VirtualMachineStageReady

		return status
	}

	status.Error = "domain has not been observed"

	return status
}

func flattenGuestAddresses(ifaces []hypervisor.VirtualMachineGuestInterfaceSpec) []string {
	return xslices.FlatMap(ifaces, func(iface hypervisor.VirtualMachineGuestInterfaceSpec) []string {
		return xslices.Map(iface.IPAddresses, netip.Prefix.String)
	})
}

func reconcileVirtualMachinePower(desired, stopMode string, status hypervisor.VirtualMachineStatusSpec) hypervisor.VirtualMachineStatusSpec {
	switch {
	case desired == hypervisorhelpers.VirtualMachinePowerStateStopped.String():
		if stopMode == hypervisorhelpers.StopModeGraceful.String() {
			status.Error = hypervisor.GracefulStopPendingError

			break
		}

		status.Error = "domain is still defined"
	default:
		if status.PowerState.String() == desired {
			status.Stage = hypervisorhelpers.VirtualMachineStageReady
		}
	}

	return status
}
