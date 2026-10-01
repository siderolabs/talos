// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// virtualMachineControllerName is the finalizer the virtual machine runtime holds on domain
// specs and placements while a domain may be running.
const virtualMachineControllerName = "hypervisor.VirtualMachineController"

// CPUPartitionController applies the desired CPU partition policy to the kernel.
//
// It is the only writer of cpuset.cpus on the fixed roots and the virtual machine partitions.
// A desired change is classified by cpupartition.Plan against the applied policy and the
// virtual machines actually occupying their partitions; only a live-safe plan is executed, one
// verified step at a time, and a blocked transition leaves the kernel, the applied status, the
// kubelet reservation and every placement untouched. The controller never stops, pauses or
// restarts a virtual machine.
//
// Admission protocol with the virtual machine runtime: before snapshotting occupancy for a plan
// that will write, the coordinator publishes Phase=applying (admission closed). The runtime
// claims the domain spec and placement before it reads the phase; a start that read an open
// phase therefore already has a claim the snapshot sees, and a claim taken after the close reads
// the closed phase and does not start. Existing domains and claims are untouched while closed.
type CPUPartitionController struct {
	V1Alpha1Mode machineruntime.Mode
	// FS is the cgroup/sysfs access; nil selects the host.
	FS cpupartition.CgroupFS
	// PollInterval bounds how long a barrier (kubepods release, a cgroup not yet created,
	// populated=0) waits between re-checks; a pending barrier also wakes on every input event.
	PollInterval time.Duration
	// BarrierTimeout bounds how long a plan waits on one barrier before it is reported as a failed
	// transition; the applied state is kept for retry or revert.
	BarrierTimeout time.Duration

	barrierSince time.Time
	barrier      string
}

// Name implements controller.Controller interface.
func (ctrl *CPUPartitionController) Name() string {
	return "runtime.CPUPartitionController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CPUPartitionController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.CPUPartitionSpecType,
			ID:        optional.Some(runtime.CPUPartitionSpecID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineCPUPlacementType,
			Kind:      controller.InputDestroyReady,
		},
		{
			// Wake-up only: the online set is read from sysfs.
			Namespace: hardware.NamespaceName,
			Type:      hardware.CPUCoreType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CPUPartitionController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.CPUPartitionStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: k8s.KubeletCPUReservationType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: hypervisor.VirtualMachineCPUPlacementType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CPUPartitionController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.FS == nil {
		ctrl.FS = cpupartition.HostFS{CgroupRoot: constants.CgroupMountPath, SysfsPath: "/sys"}
	}

	if ctrl.PollInterval == 0 {
		ctrl.PollInterval = 2 * time.Second
	}

	if ctrl.BarrierTimeout == 0 {
		ctrl.BarrierTimeout = 5 * time.Minute
	}

	var poll <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-poll:
		}

		pending, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			return err
		}

		// Poll only while a barrier is pending: no heartbeat otherwise.
		poll = nil
		if pending {
			poll = time.After(ctrl.PollInterval)
		}

		r.ResetRestartBackoff()
	}
}

// reconcile returns true when a barrier is pending and the controller must re-check.
func (ctrl *CPUPartitionController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (bool, error) {
	spec, err := safe.ReaderGetByID[*runtime.CPUPartitionSpec](ctx, r, runtime.CPUPartitionSpecID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("error getting CPU partition spec: %w", err)
	}

	if spec == nil {
		// Projection pending: nothing is known yet, publish nothing.
		return false, nil
	}

	if ctrl.V1Alpha1Mode.InContainer() {
		// The projection already disabled the policy; the kubelet only needs to learn it is unmanaged.
		return false, ctrl.publishReservation(ctx, r, nil)
	}

	status, err := safe.ReaderGetByID[*runtime.CPUPartitionStatus](ctx, r, runtime.CPUPartitionStatusID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("error getting CPU partition status: %w", err)
	}

	if !spec.TypedSpec().Enabled && status == nil {
		// Confirmed no policy and nothing applied: zero filesystem access.
		return false, errors.Join(ctrl.publishReservation(ctx, r, nil), ctrl.cleanupPlacements(ctx, r, nil))
	}

	return ctrl.converge(ctx, r, logger, spec, status)
}

// reconcileState is everything one converge pass has observed.
type reconcileState struct {
	spec      *runtime.CPUPartitionSpec
	status    *runtime.CPUPartitionStatus
	applied   runtime.CPUPartitionStatusSpec
	online    cpuset.CPUSet
	desired   cpupartition.Policy
	consumers []cpupartition.Consumer
	// placements are the existing placement resources by virtual machine name.
	placements map[string]*hypervisor.VirtualMachineCPUPlacement
}

// converge computes the plan from the applied policy to the desired one and executes it.
func (ctrl *CPUPartitionController) converge(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	spec *runtime.CPUPartitionSpec, status *runtime.CPUPartitionStatus,
) (bool, error) {
	rs, result, err := ctrl.observe(ctx, r, spec, status)
	if err != nil {
		return false, err
	}

	if result.IsBlocked() {
		return false, ctrl.publishBlocked(ctx, r, rs, result)
	}

	if len(result.Steps) > 0 {
		// Close admission, then snapshot occupancy again: a claim taken before the close is now
		// visible, a claim taken after it reads the closed phase and waits.
		if err = ctrl.closeAdmission(ctx, r, rs); err != nil {
			return false, err
		}

		if rs, result, err = ctrl.observe(ctx, r, spec, rs.status); err != nil {
			return false, err
		}

		if result.IsBlocked() {
			return false, ctrl.publishBlocked(ctx, r, rs, result)
		}

		// A failed step is recorded in the status and retried by polling, like a barrier: the
		// applied state is kept, nothing is granted, and the kernel is re-read on the retry.
		if err = ctrl.execute(ctx, r, logger, rs, result.Steps); err != nil {
			return false, err
		}

		if rs.applied.Phase == runtime.CPUPartitionPhaseConverging {
			return true, ctrl.retainPlacements(ctx, r, rs.placements)
		}
	}

	return ctrl.finish(ctx, r, rs)
}

// finish runs once the applied policy equals the desired one: restoration when the policy is
// gone, admission and ready otherwise.
func (ctrl *CPUPartitionController) finish(ctx context.Context, r controller.Runtime, rs *reconcileState) (bool, error) {
	ctrl.barrier, ctrl.barrierSince = "", time.Time{}
	rs.applied.Error, rs.applied.Waiting, rs.applied.Blocked = "", "", nil
	rs.applied.Exclusive = exclusiveSlices(rs.spec)

	if rs.desired.KubeletReservation == nil {
		if err := ctrl.publishReservation(ctx, r, nil); err != nil {
			return false, err
		}
	}

	if !rs.spec.TypedSpec().Enabled {
		return ctrl.restore(ctx, r, rs)
	}

	if err := ctrl.admit(ctx, r, rs); err != nil {
		return false, err
	}

	rs.applied.Phase = runtime.CPUPartitionPhaseReady

	return false, ctrl.publishStatus(ctx, r, rs, rs.applied)
}

// observe reads the world and classifies the transition; it performs no writes.
func (ctrl *CPUPartitionController) observe(ctx context.Context, r controller.Runtime,
	spec *runtime.CPUPartitionSpec, status *runtime.CPUPartitionStatus,
) (*reconcileState, cpupartition.Result, error) {
	rs := &reconcileState{spec: spec, status: status}

	var err error

	if rs.online, err = ctrl.FS.Online(); err != nil {
		return nil, cpupartition.Result{}, fmt.Errorf("error reading online CPUs: %w", err)
	}

	if status != nil {
		rs.applied = status.TypedSpec().DeepCopy()
	}

	// Kernel writes and status publication are not atomic: a write whose status publication
	// failed is reconciled from the kernel before anything else is trusted.
	drift, err := ctrl.recoverApplied(&rs.applied)
	if err != nil {
		return nil, cpupartition.Result{}, err
	}

	if rs.consumers, rs.placements, err = ctrl.observeConsumers(ctx, r, spec); err != nil {
		return nil, cpupartition.Result{}, err
	}

	rs.desired = desiredPolicy(spec, rs.online)

	applied := appliedPolicy(&rs.applied)

	// The applied kubelet reservation is the last staged command this controller published: it
	// may differ from the desired one while a barrier is pending, on removal and on recovery,
	// so it is read back rather than derived.
	if applied.KubeletReservation, err = ctrl.appliedReservation(ctx, r); err != nil {
		return nil, cpupartition.Result{}, err
	}

	result := cpupartition.Plan(cpupartition.Input{
		Applied:   applied,
		Desired:   rs.desired,
		Consumers: rs.consumers,
		Online:    rs.online,
	})

	rs.applied.EnforcementLoss = nil

	for _, target := range slices.Concat(result.EnforcementLoss, drift) {
		if !slices.Contains(rs.applied.EnforcementLoss, target.Key()) {
			rs.applied.EnforcementLoss = append(rs.applied.EnforcementLoss, target.Key())
		}
	}

	slices.Sort(rs.applied.EnforcementLoss)

	// A boundary known to be invalid blocks the transition and admission alike: no new virtual
	// machine starts on it, nothing is written, and the loss is named in the status.
	for _, key := range rs.applied.EnforcementLoss {
		result.Blocked = append(result.Blocked, cpupartition.Block{
			Reason: fmt.Sprintf("%s: enforcement lost (offline CPUs or mask changed outside Talos); "+
				"new starts are refused and the isolation of running virtual machines may no longer hold until it is restored", key),
		})
	}

	return rs, result, nil
}

// publishBlocked records why the desired policy is not applied; nothing applied changes and
// existing placements are kept.
func (ctrl *CPUPartitionController) publishBlocked(ctx context.Context, r controller.Runtime, rs *reconcileState, result cpupartition.Result) error {
	rs.applied.Phase = runtime.CPUPartitionPhaseBlocked
	rs.applied.Blocked = nil
	rs.applied.Error, rs.applied.Waiting = "", ""

	for _, block := range result.Blocked {
		rs.applied.Blocked = append(rs.applied.Blocked, runtime.CPUPartitionBlock{
			Reason:          block.Reason,
			VirtualMachines: block.Consumers,
			CPUs:            block.CPUs.String(),
		})
	}

	return errors.Join(ctrl.publishStatus(ctx, r, rs, rs.applied), ctrl.retainPlacements(ctx, r, rs.placements))
}

// closeAdmission publishes Phase=applying; once the write returned every later read of the
// status sees it closed (COSI writes are linearizable through the state's collection lock).
func (ctrl *CPUPartitionController) closeAdmission(ctx context.Context, r controller.Runtime, rs *reconcileState) error {
	if rs.applied.Phase == runtime.CPUPartitionPhaseApplying {
		return nil
	}

	rs.applied.Phase = runtime.CPUPartitionPhaseApplying

	return ctrl.publishStatus(ctx, r, rs, rs.applied)
}

// publishStatus writes the status and keeps the reconcile state's copy in sync.
func (ctrl *CPUPartitionController) publishStatus(ctx context.Context, r controller.Runtime, rs *reconcileState, applied runtime.CPUPartitionStatusSpec) error {
	res, err := safe.WriterModifyWithResult(ctx, r, runtime.NewCPUPartitionStatus(), func(res *runtime.CPUPartitionStatus) error {
		*res.TypedSpec() = applied.DeepCopy()

		return nil
	})
	if err != nil {
		return fmt.Errorf("error publishing CPU partition status: %w", err)
	}

	rs.status = res
	rs.applied = applied.DeepCopy()

	return nil
}

// appliedReservation reads the reservation last published by this controller; nil means
// unmanaged (or not published yet, which makes the first publication a step).
func (ctrl *CPUPartitionController) appliedReservation(ctx context.Context, r controller.Runtime) (*cpuset.CPUSet, error) {
	reservation, err := safe.ReaderGetByID[*k8s.KubeletCPUReservation](ctx, r, k8s.KubeletID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error getting kubelet CPU reservation: %w", err)
	}

	if !reservation.TypedSpec().Managed {
		return nil, nil
	}

	reserved, err := cpuset.Parse(reservation.TypedSpec().ReservedCPUs)
	if err != nil {
		return nil, fmt.Errorf("invalid published kubelet CPU reservation %q: %w", reservation.TypedSpec().ReservedCPUs, err)
	}

	return &reserved, nil
}

func (ctrl *CPUPartitionController) publishReservation(ctx context.Context, r controller.Runtime, reserved *cpuset.CPUSet) error {
	return safe.WriterModify(ctx, r, k8s.NewKubeletCPUReservation(), func(res *k8s.KubeletCPUReservation) error {
		if reserved == nil {
			*res.TypedSpec() = k8s.KubeletCPUReservationSpec{}
		} else {
			*res.TypedSpec() = k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: reserved.String()}
		}

		return nil
	})
}

// desiredPolicy projects the spec into classifier terms; the shared remainder and the kubelet
// reservation (online minus kubepods) are derived here.
func desiredPolicy(spec *runtime.CPUPartitionSpec, online cpuset.CPUSet) cpupartition.Policy {
	policy := cpupartition.Policy{Sets: map[cpupartition.Target]cpuset.CPUSet{}, Exclusive: map[string]bool{}}

	if !spec.TypedSpec().Enabled {
		return policy
	}

	for root, list := range spec.TypedSpec().Roots {
		set, err := cpuset.Parse(list)
		if err != nil {
			continue
		}

		policy.Sets[cpupartition.Root(config.CPUPartitionRoot(root))] = set
	}

	if kubepods, ok := policy.Sets[cpupartition.Root(config.CPUPartitionRootKubepods)]; ok {
		reserved := online.Difference(kubepods)
		policy.KubeletReservation = &reserved
	}

	vmRoot, managed := policy.Sets[cpupartition.Root(config.CPUPartitionRootVirtualMachines)]
	if !managed {
		return policy
	}

	remainder := vmRoot

	for _, slice := range spec.TypedSpec().Slices {
		set, err := cpuset.Parse(slice.CPUs)
		if err != nil {
			continue
		}

		policy.Sets[cpupartition.Slice(slice.Name)] = set
		policy.Exclusive[slice.Name] = slice.Exclusive
		remainder = remainder.Difference(set)
	}

	if !remainder.IsEmpty() {
		policy.Sets[cpupartition.Shared] = remainder
	}

	return policy
}

func appliedPolicy(applied *runtime.CPUPartitionStatusSpec) cpupartition.Policy {
	policy := cpupartition.Policy{Sets: map[cpupartition.Target]cpuset.CPUSet{}, Exclusive: map[string]bool{}}

	for _, entry := range applied.Targets {
		target, ok := cpupartition.ParseKey(entry.Key)
		if !ok || entry.LastApplied == "" {
			continue
		}

		set, err := cpuset.Parse(entry.LastApplied)
		if err != nil {
			continue
		}

		policy.Sets[target] = set
	}

	for _, name := range applied.Exclusive {
		policy.Exclusive[name] = true
	}

	return policy
}

func desiredManagesVirtualMachines(spec *runtime.CPUPartitionSpec) bool {
	_, ok := spec.TypedSpec().Roots[string(config.CPUPartitionRootVirtualMachines)]

	return spec.TypedSpec().Enabled && ok
}

func desiredTarget(spec *runtime.CPUPartitionSpec, slice string) cpupartition.Target {
	switch {
	case !desiredManagesVirtualMachines(spec):
		return cpupartition.Root(config.CPUPartitionRootVirtualMachines)
	case slice == "":
		return cpupartition.Shared
	default:
		return cpupartition.Slice(slice)
	}
}

func exclusiveSlices(spec *runtime.CPUPartitionSpec) []string {
	var names []string

	for _, slice := range spec.TypedSpec().Slices {
		if slice.Exclusive {
			names = append(names, slice.Name)
		}
	}

	return names
}

func placementKey(placement *hypervisor.VirtualMachineCPUPlacementSpec) string {
	switch {
	case placement.Partition == "/"+constants.CgroupVirtualMachines:
		return cpupartition.Root(config.CPUPartitionRootVirtualMachines).Key()
	case placement.Slice == "":
		return cpupartition.Shared.Key()
	default:
		return cpupartition.Slice(placement.Slice).Key()
	}
}

func pinsOf(cpu hypervisor.VirtualMachineCPUSpec) cpuset.CPUSet {
	pins := cpuset.New()

	for _, pin := range cpu.Pins {
		if set, err := hypervisorhelpers.ParseHostIDList(pin.CPUs, hypervisorhelpers.MaxHostCPUID); err == nil {
			pins = pins.Union(set)
		}
	}

	if set, err := hypervisorhelpers.ParseHostIDList(cpu.EmulatorPin, hypervisorhelpers.MaxHostCPUID); err == nil {
		pins = pins.Union(set)
	}

	return pins
}

// retainPlacements keeps every existing placement as it is (blocked/converging: no admission changes).
func (ctrl *CPUPartitionController) retainPlacements(ctx context.Context, r controller.Runtime, placements map[string]*hypervisor.VirtualMachineCPUPlacement) error {
	keep := map[string]struct{}{}
	for name := range placements {
		keep[name] = struct{}{}
	}

	return ctrl.cleanupPlacements(ctx, r, keep)
}

// cleanupPlacements tears down every placement not in keep and destroys the released ones.
func (ctrl *CPUPartitionController) cleanupPlacements(ctx context.Context, r controller.Runtime, keep map[string]struct{}) error {
	placements, err := safe.ReaderListAll[*hypervisor.VirtualMachineCPUPlacement](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing placements: %w", err)
	}

	for placement := range placements.All() {
		if _, ok := keep[placement.Metadata().ID()]; ok && placement.Metadata().Phase() == resource.PhaseRunning {
			continue
		}

		ready, err := r.Teardown(ctx, placement.Metadata())
		if err != nil {
			return fmt.Errorf("error tearing down placement %q: %w", placement.Metadata().ID(), err)
		}

		if !ready {
			continue
		}

		if err = r.Destroy(ctx, placement.Metadata()); err != nil {
			return fmt.Errorf("error destroying placement %q: %w", placement.Metadata().ID(), err)
		}
	}

	return nil
}
