// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const virtqemudServiceID = "ext-virtqemud"

// VirtualMachineController reconciles running transient domains with virtqemud.
type VirtualMachineController struct {
	V1Alpha1Mode machineruntime.Mode
	Open         func(context.Context) (libvirtdomain.Client, error)
}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineController) Name() string {
	return "hypervisor.VirtualMachineController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VirtualMachineController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainSpecType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.SystemInformationType,
			ID:        optional.Some(hardware.SystemInformationID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
			// Strong: this controller holds every disk status its domains read from, and is the
			// only one which can give such a hold back.
			Kind: controller.InputStrong,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(virtqemudServiceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VirtualMachineController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		ready, err := virtqemudReady(ctx, runtime)
		if err != nil {
			return err
		}

		if !ready {
			runtime.ResetRestartBackoff()

			continue
		}

		if err = ctrl.reconcile(ctx, runtime, logger); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func virtqemudReady(ctx context.Context, reader controller.Reader) (bool, error) {
	service, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, reader, virtqemudServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("get %q service: %w", virtqemudServiceID, err)
	}

	return service.TypedSpec().Running && (service.TypedSpec().Healthy || service.TypedSpec().Unknown), nil
}

//nolint:gocyclo
func (ctrl *VirtualMachineController) reconcile(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine domain specs: %w", err)
	}

	known := make(map[string]struct{}, specs.Len())

	for spec := range specs.All() {
		known[spec.Metadata().ID()] = struct{}{}
	}

	if err = ctrl.releaseOrphanHolds(ctx, r, known); err != nil {
		return err
	}

	if specs.Len() == 0 {
		return nil
	}

	machineUUID, err := getMachineUUID(ctx, r)
	if err != nil {
		return err
	}

	client, err := ctrl.Open(ctx)
	if err != nil {
		return fmt.Errorf("waiting for QEMU daemon: %w", err)
	}
	defer client.Close()

	domains, err := client.Domains()
	if err != nil {
		return fmt.Errorf("failed to list libvirt domains: %w", err)
	}

	byName := make(map[string]libvirtdomain.Domain, len(domains))
	for _, domain := range domains {
		byName[domain.Name] = domain
	}

	var reconcileErrors error

	for spec := range specs.All() {
		switch err = ctrl.reconcileSpec(ctx, r, logger, client, machineUUID, byName, spec); {
		case errors.Is(err, errDiskNotReady):
			logger.Info("virtual machine is waiting for its disks",
				zap.String("virtual_machine", spec.Metadata().ID()), zap.Error(err))
		case err != nil:
			reconcileErrors = errors.Join(reconcileErrors, err)
		}
	}

	return reconcileErrors
}

func getMachineUUID(ctx context.Context, reader controller.Reader) (uuid.UUID, error) {
	machine, err := safe.ReaderGetByID[*hardware.SystemInformation](ctx, reader, hardware.SystemInformationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get system information: %w", err)
	}

	machineUUID, err := uuid.Parse(machine.TypedSpec().UUID)
	if err != nil || machineUUID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid machine UUID %q", machine.TypedSpec().UUID)
	}

	return machineUUID, nil
}

func (ctrl *VirtualMachineController) reconcileSpec(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	client libvirtdomain.Client, machineUUID uuid.UUID, domains map[string]libvirtdomain.Domain,
	spec *hypervisor.VirtualMachineDomainSpec,
) error {
	name := spec.Metadata().ID()
	_, exists := domains[name]
	claimed := spec.Metadata().Finalizers().Has(ctrl.Name())

	if spec.Metadata().Phase() != resource.PhaseRunning || spec.TypedSpec().PowerState == "stopped" {
		return ctrl.stopClaimed(ctx, r, logger, client, machineUUID, spec, exists, claimed)
	}

	if spec.TypedSpec().PowerState != "running" {
		return fmt.Errorf("unsupported power state %q for domain %q", spec.TypedSpec().PowerState, name)
	}

	if exists && !claimed {
		return fmt.Errorf("refusing to adopt unclaimed domain %q", name)
	}

	return ctrl.startSpec(ctx, r, client, machineUUID, spec, claimed)
}

// startSpec brings one domain up on its definition.
func (ctrl *VirtualMachineController) startSpec(ctx context.Context, r controller.ReaderWriter,
	client libvirtdomain.Client, machineUUID uuid.UUID, spec *hypervisor.VirtualMachineDomainSpec, claimed bool,
) error {
	name := spec.Metadata().ID()

	// Held before libvirt is handed the definition: a domain reads its disks from the moment it starts.
	if err := ctrl.holdDisks(ctx, r, name, spec.TypedSpec().Disks); err != nil {
		return err
	}

	// Claimed before libvirt is handed the definition, so a domain this controller started is never
	// one the spec has no claim on: the claim says a domain may exist, not that one does.
	if !claimed {
		if err := r.AddFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to claim domain spec %q: %w", name, err)
		}
	}

	if err := client.Start(libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(machineUUID, name)}, spec.TypedSpec().DomainXML); err != nil {
		return fmt.Errorf("failed to start domain %q: %w", name, err)
	}

	// Start has replaced the domain if its definition changed, so what the previous one read is now free.
	if err := ctrl.releaseDisksExcept(ctx, r, name, spec.TypedSpec().Disks); err != nil {
		return err
	}

	return nil
}

// holdDisks takes a hold on every disk status the definition attaches.
func (ctrl *VirtualMachineController) holdDisks(ctx context.Context, r controller.ReaderWriter, name string, disks []string) error {
	for _, id := range disks {
		diskStatus, err := safe.ReaderGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, r, id)

		switch {
		case state.IsNotFoundError(err):
			return fmt.Errorf("domain %q: disk status %q is %w: not published yet", name, id, errDiskNotReady)
		case err != nil:
			return fmt.Errorf("failed to get disk status %q of domain %q: %w", id, name, err)
		}

		if diskStatus.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if diskStatus.Metadata().Phase() != resource.PhaseRunning {
			return fmt.Errorf("domain %q: disk status %q is %w: going away", name, id, errDiskNotReady)
		}

		if err := r.AddFinalizer(ctx, diskStatus.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to hold disk status %q of domain %q: %w", id, name, err)
		}
	}

	return nil
}

// releaseHolds gives back every hold this controller has on a disk status keep does not name.
func (ctrl *VirtualMachineController) releaseHolds(
	ctx context.Context, r controller.ReaderWriter, keep func(*hypervisor.VirtualMachineDiskStatus) bool,
) error {
	diskStatuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine disk statuses: %w", err)
	}

	for diskStatus := range diskStatuses.All() {
		md := diskStatus.Metadata()

		if !md.Finalizers().Has(ctrl.Name()) || keep(diskStatus) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, md, ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to release disk status %q: %w", md.ID(), err)
		}
	}

	return nil
}

// releaseDisksExcept gives back the hold on every disk status of a domain which the definition now
// running does not attach.
func (ctrl *VirtualMachineController) releaseDisksExcept(ctx context.Context, r controller.ReaderWriter, name string, keep []string) error {
	return ctrl.releaseHolds(ctx, r, func(diskStatus *hypervisor.VirtualMachineDiskStatus) bool {
		return diskStatus.TypedSpec().VirtualMachine != name || slices.Contains(keep, diskStatus.Metadata().ID())
	})
}

// releaseOrphanHolds gives back the holds left on the disks of a virtual machine which has no domain
// spec any more.
func (ctrl *VirtualMachineController) releaseOrphanHolds(ctx context.Context, r controller.ReaderWriter, known map[string]struct{}) error {
	return ctrl.releaseHolds(ctx, r, func(diskStatus *hypervisor.VirtualMachineDiskStatus) bool {
		_, stillSpecified := known[diskStatus.TypedSpec().VirtualMachine]

		return stillSpecified
	})
}

func (ctrl *VirtualMachineController) stopClaimed(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	client libvirtdomain.Client,
	machineUUID uuid.UUID,
	spec *hypervisor.VirtualMachineDomainSpec,
	exists bool,
	claimed bool,
) error {
	name := spec.Metadata().ID()

	switch {
	case claimed && exists:
		if err := client.Remove(libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(machineUUID, name)}); err != nil {
			return fmt.Errorf("failed to remove domain %q: %w", name, err)
		}
	case exists:
		// A domain this controller never claimed is not its to remove, and it goes on reading
		// whatever it was given. Its disks stay held until it is gone.
		logger.Warn("keeping the disks of an unclaimed domain which is still there",
			zap.String("virtual_machine", name))

		return nil
	}

	// No domain of this virtual machine reads a disk any more. Released even when the spec was
	// never claimed: a start refused before the claim leaves holds behind no claim.
	if err := ctrl.releaseDisksExcept(ctx, r, name, nil); err != nil {
		return err
	}

	if !claimed {
		return nil
	}

	if err := r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("failed to release domain spec %q: %w", name, err)
	}

	return nil
}
