// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"
	"libvirt.org/go/libvirtxml"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const virtqemudServiceID = "ext-virtqemud"

// VirtualMachineController reconciles running transient domains with virtqemud.
//
// While the CPU partition policy manages virtual machines (desired, or still applied), a
// domain starts only with a granted VirtualMachineCPUPlacement whose partition the rendered
// definition carries and whose applied CPUs are verified, and the placement is claimed before
// the start and released only after the domain was actually removed: a claim is proof the
// partition may have tasks. The claims (domain spec, then placement) are taken before the
// policy phase is read, so a start racing the coordinator either has a claim the coordinator's
// occupancy snapshot sees, or reads the closed phase and waits. A missing CPUPartitionSpec means
// the projection is pending, not that there is no policy: starts wait, stops proceed. The
// controller never stops a domain because a policy changed.
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
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(virtqemudServiceID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineCPUPlacementType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.CPUPartitionSpecType,
			ID:        optional.Some(runtime.CPUPartitionSpecID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.CPUPartitionStatusType,
			ID:        optional.Some(runtime.CPUPartitionStatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VirtualMachineController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
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

		if err = ctrl.reconcile(ctx, runtime); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func virtqemudReady(ctx context.Context, runtime controller.Runtime) (bool, error) {
	service, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, runtime, virtqemudServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("get %q service: %w", virtqemudServiceID, err)
	}

	return service.TypedSpec().Running && (service.TypedSpec().Healthy || service.TypedSpec().Unknown), nil
}

func (ctrl *VirtualMachineController) reconcile(ctx context.Context, runtime controller.Runtime) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainSpec](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine domain specs: %w", err)
	}

	if specs.Len() == 0 {
		return nil
	}

	machineUUID, err := getMachineUUID(ctx, runtime)
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
		if err = ctrl.reconcileSpec(ctx, runtime, client, machineUUID, byName, spec); err != nil {
			reconcileErrors = errors.Join(reconcileErrors, err)
		}
	}

	return reconcileErrors
}

// cpuPolicy is what the runtime knows about the CPU partition policy.
type cpuPolicy struct {
	// pending: the projection has not been published yet; nothing may start.
	pending bool
	// guarded: virtual machines are placed by the coordinator (desired or still applied).
	guarded bool
	// closed: the coordinator is snapshotting occupancy or writing; no new start.
	closed bool
	// lost: a managed boundary is known invalid (offline CPUs, foreign change); no new start.
	lost bool
	// allocations are the applied, verified CPU masks per placement key, for the pin check.
	allocations map[string]cpuset.CPUSet
	placements  map[string]*hypervisor.VirtualMachineCPUPlacement
}

func (ctrl *VirtualMachineController) observePolicy(ctx context.Context, r controller.Runtime) (cpuPolicy, error) {
	policy := cpuPolicy{allocations: map[string]cpuset.CPUSet{}, placements: map[string]*hypervisor.VirtualMachineCPUPlacement{}}

	spec, err := safe.ReaderGetByID[*runtime.CPUPartitionSpec](ctx, r, runtime.CPUPartitionSpecID)
	if err != nil && !state.IsNotFoundError(err) {
		return policy, fmt.Errorf("failed to get CPU partition spec: %w", err)
	}

	if spec == nil {
		policy.pending = true

		return policy, nil
	}

	if _, managed := spec.TypedSpec().Roots[string(config.CPUPartitionRootVirtualMachines)]; managed && spec.TypedSpec().Enabled {
		policy.guarded = true
	}

	if err = ctrl.observeApplied(ctx, r, &policy); err != nil {
		return policy, err
	}

	placements, err := safe.ReaderListAll[*hypervisor.VirtualMachineCPUPlacement](ctx, r)
	if err != nil {
		return policy, fmt.Errorf("failed to list placements: %w", err)
	}

	for placement := range placements.All() {
		policy.placements[placement.Metadata().ID()] = placement
	}

	return policy, nil
}

// observeApplied reads the applied policy: an applied virtual machine boundary keeps the guard on
// after the desired policy was removed, until the coordinator has restored everything.
func (ctrl *VirtualMachineController) observeApplied(ctx context.Context, r controller.Runtime, policy *cpuPolicy) error {
	status, err := safe.ReaderGetByID[*runtime.CPUPartitionStatus](ctx, r, runtime.CPUPartitionStatusID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get CPU partition status: %w", err)
	}

	policy.closed = !status.TypedSpec().Phase.AdmissionOpen()
	policy.lost = len(status.TypedSpec().EnforcementLoss) > 0

	for _, target := range status.TypedSpec().Targets {
		if strings.HasPrefix(target.Key, string(config.CPUPartitionRootVirtualMachines)) {
			policy.guarded = true
		}

		// Only a completed write is an allocation: a pending intent is not verified yet.
		if target.Intended != "" && target.Intended != target.LastApplied {
			continue
		}

		if set, parseErr := cpuset.Parse(target.LastApplied); parseErr == nil && !set.IsEmpty() {
			policy.allocations[target.Key] = set
		}
	}

	return nil
}

func getMachineUUID(ctx context.Context, runtime controller.Runtime) (uuid.UUID, error) {
	machine, err := safe.ReaderGetByID[*hardware.SystemInformation](ctx, runtime, hardware.SystemInformationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get system information: %w", err)
	}

	machineUUID, err := uuid.Parse(machine.TypedSpec().UUID)
	if err != nil || machineUUID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid machine UUID %q", machine.TypedSpec().UUID)
	}

	return machineUUID, nil
}

func (ctrl *VirtualMachineController) reconcileSpec(ctx context.Context, runtime controller.Runtime,
	client libvirtdomain.Client, machineUUID uuid.UUID, domains map[string]libvirtdomain.Domain,
	spec *hypervisor.VirtualMachineDomainSpec,
) error {
	name := spec.Metadata().ID()
	_, exists := domains[name]
	claimed := spec.Metadata().Finalizers().Has(ctrl.Name())

	policy, err := ctrl.observePolicy(ctx, runtime)
	if err != nil {
		return err
	}

	if spec.Metadata().Phase() != resource.PhaseRunning || spec.TypedSpec().PowerState == "stopped" {
		return ctrl.stopClaimed(ctx, runtime, client, machineUUID, spec, policy.placements[name], exists, claimed)
	}

	if spec.TypedSpec().PowerState != "running" {
		return fmt.Errorf("unsupported power state %q for domain %q", spec.TypedSpec().PowerState, name)
	}

	if exists && !claimed {
		return fmt.Errorf("refusing to adopt unclaimed domain %q", name)
	}

	if policy.pending {
		return fmt.Errorf("domain %q: waiting for the CPU partition projection", name)
	}

	if err = ctrl.claimForStart(ctx, runtime, policy, spec, exists, claimed); err != nil {
		return err
	}

	if err = client.Start(libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(machineUUID, name)}, spec.TypedSpec().DomainXML); err != nil {
		// The claims are kept: a failed start does not prove no QEMU was created.
		return fmt.Errorf("failed to start domain %q: %w", name, err)
	}

	return nil
}

// claimForStart takes the claims a start needs. Claims are the coordinator's evidence of a start
// in flight and are taken before the policy is re-read: a claim the coordinator's snapshot
// misses was taken after admission closed and then reads the closed phase. Under a placing
// policy the placement is claimed first, so a start merely waiting for its placement never
// holds a bare domain claim (which would look like a machine running in the root).
func (ctrl *VirtualMachineController) claimForStart(ctx context.Context, runtime controller.Runtime,
	policy cpuPolicy, spec *hypervisor.VirtualMachineDomainSpec, exists, claimed bool,
) error {
	name := spec.Metadata().ID()

	if policy.guarded {
		if claimed && !exists {
			// A domain claim without a domain under a placing policy is stale evidence from a root
			// start which never happened (its Start failed, or the policy flipped underneath
			// it); it is released so the coordinator does not plan around a phantom machine.
			if err := runtime.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("failed to release domain spec %q: %w", name, err)
			}

			claimed = false
		}

		if err := ctrl.claimPlacement(ctx, runtime, spec, policy.placements[name], exists); err != nil {
			return err
		}
	}

	if !claimed {
		if err := runtime.AddFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to claim domain spec %q: %w", name, err)
		}
	}

	if policy.guarded || exists {
		return nil
	}

	return ctrl.recheckRootStart(ctx, runtime, spec)
}

// recheckRootStart re-reads the policy after a root start's domain claim: the policy may have
// started placing machines since it was read; the claim is then stale evidence and is released
// before anything is started.
func (ctrl *VirtualMachineController) recheckRootStart(ctx context.Context, runtime controller.Runtime, spec *hypervisor.VirtualMachineDomainSpec) error {
	name := spec.Metadata().ID()

	current, err := ctrl.observePolicy(ctx, runtime)
	if err != nil {
		return err
	}

	if !current.guarded && !current.closed {
		return nil
	}

	if err = runtime.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("failed to release domain spec %q: %w", name, err)
	}

	return fmt.Errorf("domain %q: CPU partition policy changed while starting; retrying", name)
}

// claimPlacement makes sure the domain starts in the partition the coordinator granted: the
// placement must be granted (not being torn down), the definition must carry its partition, its
// pins must fit the partition's verified applied CPUs, and the claim is taken before the policy
// phase is re-read, so a claim taken after the coordinator closed admission reads the closed
// phase and waits, while a claim taken before is seen by the coordinator's snapshot.
func (ctrl *VirtualMachineController) claimPlacement(ctx context.Context, runtime controller.Runtime,
	spec *hypervisor.VirtualMachineDomainSpec, placement *hypervisor.VirtualMachineCPUPlacement, exists bool,
) error {
	name := spec.Metadata().ID()

	pins, held, err := placementMatches(ctrl.Name(), spec, placement)
	if err != nil {
		return err
	}

	if held && exists {
		// The claim belongs to the domain libvirt lists: it is running, admitted, and the
		// coordinator plans around it; a closed phase or a later loss does not stop it, only
		// the operator's stop does.
		return nil
	}

	if !held {
		if err = runtime.AddFinalizer(ctx, placement.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to claim CPU placement %q: %w", name, err)
		}
	}

	// Re-read after the claim: only a phase observed after the claim is authoritative. A claim
	// refused here is released again before anything was started: the coordinator must not
	// plan around a start which never happened, and the runtime retries under an open phase.
	//
	// A claim already held without a listed domain is different: it is the evidence of a start
	// whose outcome the runtime does not know (a failed Start, or a guest which went away), so
	// it stays, the coordinator keeps treating the CPUs as possibly occupied, and nothing new is
	// started until admission is open again; the claim is not an authorisation to start another
	// domain.
	current, err := ctrl.observePolicy(ctx, runtime)
	if err != nil {
		return err
	}

	if err = admissionCheck(current, placement, pins); err != nil {
		if !held {
			if releaseErr := runtime.RemoveFinalizer(ctx, placement.Metadata(), ctrl.Name()); releaseErr != nil {
				return fmt.Errorf("failed to release CPU placement %q: %w", name, releaseErr)
			}
		}

		return fmt.Errorf("domain %q: %w", name, err)
	}

	return nil
}

// placementMatches checks that a granted placement exists for the definition and returns the
// definition's pins and whether the runtime already holds the placement.
func placementMatches(finalizer string, spec *hypervisor.VirtualMachineDomainSpec, placement *hypervisor.VirtualMachineCPUPlacement) (cpuset.CPUSet, bool, error) {
	name := spec.Metadata().ID()

	if placement == nil {
		return cpuset.New(), false, fmt.Errorf("domain %q: waiting for a CPU placement", name)
	}

	held := placement.Metadata().Finalizers().Has(finalizer)

	if placement.Metadata().Phase() != resource.PhaseRunning && !held {
		return cpuset.New(), false, fmt.Errorf("domain %q: its CPU placement is being withdrawn", name)
	}

	partition, pins, err := parseDomainPlacement(spec.TypedSpec().DomainXML)
	if err != nil {
		return cpuset.New(), false, fmt.Errorf("domain %q: %w", name, err)
	}

	if partition != placement.TypedSpec().Partition {
		return cpuset.New(), false, fmt.Errorf("domain %q: definition partition %q does not match the granted %q yet", name, partition, placement.TypedSpec().Partition)
	}

	return pins, held, nil
}

// admissionCheck is the runtime's view of the coordinator's admission: open, enforced, and with
// a verified allocation the pins fit in.
func admissionCheck(policy cpuPolicy, placement *hypervisor.VirtualMachineCPUPlacement, pins cpuset.CPUSet) error {
	switch {
	case policy.closed:
		return errors.New("CPU partition admission is closed while the coordinator applies a change")
	case policy.lost:
		return errors.New("a CPU partition boundary lost enforcement; admission is closed until it is restored")
	}

	allocation, ok := policy.allocations[placementKey(placement.TypedSpec())]
	if !ok {
		return fmt.Errorf("the applied CPUs of %s are not verified", placement.TypedSpec().Partition)
	}

	if !pins.IsSubsetOf(allocation) {
		return fmt.Errorf("pins %q fall outside the applied CPUs %q of %s", pins, allocation, placement.TypedSpec().Partition)
	}

	return nil
}

// placementKey mirrors the coordinator's status key for a placement.
func placementKey(placement *hypervisor.VirtualMachineCPUPlacementSpec) string {
	switch {
	case placement.Partition == "/"+constants.CgroupVirtualMachines:
		return string(config.CPUPartitionRootVirtualMachines)
	case placement.Slice == "":
		return string(config.CPUPartitionRootVirtualMachines) + "/shared"
	default:
		return string(config.CPUPartitionRootVirtualMachines) + "/" + placement.Slice
	}
}

// parseDomainPlacement extracts the partition and the union of host CPU pins from a definition.
func parseDomainPlacement(domainXML string) (string, cpuset.CPUSet, error) {
	var domain libvirtxml.Domain

	if err := domain.Unmarshal(domainXML); err != nil {
		return "", cpuset.New(), fmt.Errorf("invalid domain XML: %w", err)
	}

	partition := ""
	if domain.Resource != nil {
		partition = domain.Resource.Partition
	}

	pins := cpuset.New()

	if domain.CPUTune != nil {
		for _, pin := range domain.CPUTune.VCPUPin {
			set, err := hypervisorhelpers.ParseHostIDList(pin.CPUSet, hypervisorhelpers.MaxHostCPUID)
			if err != nil {
				return "", cpuset.New(), fmt.Errorf("invalid vcpu pin %q: %w", pin.CPUSet, err)
			}

			pins = pins.Union(set)
		}

		if domain.CPUTune.EmulatorPin != nil {
			set, err := hypervisorhelpers.ParseHostIDList(domain.CPUTune.EmulatorPin.CPUSet, hypervisorhelpers.MaxHostCPUID)
			if err != nil {
				return "", cpuset.New(), fmt.Errorf("invalid emulator pin %q: %w", domain.CPUTune.EmulatorPin.CPUSet, err)
			}

			pins = pins.Union(set)
		}
	}

	return partition, pins, nil
}

// stopClaimed removes the domain and only then releases the claims.
func (ctrl *VirtualMachineController) stopClaimed(ctx context.Context, runtime controller.Runtime,
	client libvirtdomain.Client, machineUUID uuid.UUID, spec *hypervisor.VirtualMachineDomainSpec,
	placement *hypervisor.VirtualMachineCPUPlacement, exists, claimed bool,
) error {
	name := spec.Metadata().ID()
	placementHeld := placement != nil && placement.Metadata().Finalizers().Has(ctrl.Name())

	if !claimed && !placementHeld {
		return nil
	}

	if exists {
		if err := client.Remove(libvirtdomain.Domain{Name: name, UUID: libvirtdomain.UUID(machineUUID, name)}); err != nil {
			return fmt.Errorf("failed to remove domain %q: %w", name, err)
		}
	}

	// The placement is released first: a crash in between leaves the domain spec claimed, which
	// the next reconcile releases, whereas a placement claimed by no domain spec would be orphaned.
	if placementHeld {
		if err := runtime.RemoveFinalizer(ctx, placement.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to release CPU placement %q: %w", name, err)
		}
	}

	if claimed {
		if err := runtime.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to release domain spec %q: %w", name, err)
		}
	}

	return nil
}
