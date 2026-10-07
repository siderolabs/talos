// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// virtualMachineRuntimeName is the finalizer the virtual machine runtime holds on domain specs
// and placements while a domain may exist.
const virtualMachineRuntimeName = "hypervisor.VirtualMachineController"

// CPUPartitionController applies the desired CPU partition policy to the kernel.
//
// It is the only writer of cpuset.cpus on the fixed roots and the virtual machine partitions, and
// the producer of the kubelet CPU reservation and of virtual machine CPU placements. It never
// stops, pauses or restarts a virtual machine.
//
// Admission protocol: the virtual machine runtime claims a domain spec or placement (finalizer)
// before it reads the status phase, and starts only while the phase admits and no enforcement
// loss is reported. The controller publishes a closed phase before it takes the occupancy
// snapshot that any write, grant or withdrawal relies on, so a claim taken before the close is in
// the snapshot and a claim taken after it reads the closed phase.
type CPUPartitionController struct {
	V1Alpha1Mode machineruntime.Mode
	// FS defaults to the host cgroup and sysfs mounts.
	FS cpupartition.CgroupFS
	// PollInterval is the re-check period while a transition waits on the filesystem.
	PollInterval time.Duration
	// BarrierTimeout reports a barrier pending for longer as an error; it keeps waiting.
	BarrierTimeout time.Duration

	barrier      string
	barrierSince time.Time
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
			Namespace: k8s.NamespaceName,
			Type:      k8s.KubeletSpecType,
			ID:        optional.Some(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.KubeletCPUObservationType,
			ID:        optional.Some(k8s.KubeletID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDomainStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineCPUPlacementType,
			Kind:      controller.InputWeak,
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

		// Only a transition waiting on the filesystem polls; resource changes arrive as events.
		poll = nil
		if pending {
			poll = time.After(ctrl.PollInterval)
		}

		r.ResetRestartBackoff()
	}
}

// reconcile returns true while the transition waits on a filesystem change.
func (ctrl *CPUPartitionController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (bool, error) {
	spec, err := safe.ReaderGetByID[*runtime.CPUPartitionSpec](ctx, r, runtime.CPUPartitionSpecID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("error getting CPU partition spec: %w", err)
	}

	if spec == nil {
		// Projection pending: nothing is known yet.
		return false, nil
	}

	if ctrl.V1Alpha1Mode.InContainer() {
		return false, ctrl.publishReservation(ctx, r, nil)
	}

	status, err := safe.ReaderGetByID[*runtime.CPUPartitionStatus](ctx, r, runtime.CPUPartitionStatusID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("error getting CPU partition status: %w", err)
	}

	if !spec.TypedSpec().Enabled && status == nil {
		// No policy and nothing applied: no filesystem access at all.
		return false, errors.Join(ctrl.publishReservation(ctx, r, nil), cleanupPlacements(ctx, r, nil))
	}

	return ctrl.converge(ctx, r, logger, spec, status)
}

func (ctrl *CPUPartitionController) converge(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	spec *runtime.CPUPartitionSpec, status *runtime.CPUPartitionStatus,
) (bool, error) {
	obs, err := ctrl.observe(ctx, r, spec, status)
	if err != nil {
		return false, err
	}

	if rejected, err := ctrl.rejected(ctx, r, obs); rejected {
		return obs.waitsOnKernel(), err
	}

	if obs.settled() {
		return ctrl.finish(ctx, r, obs, false)
	}

	// Close admission, then take the authoritative snapshot: a claim taken before the close is
	// visible now, a claim taken after it reads the closed phase.
	if err = ctrl.closeAdmission(ctx, r, obs); err != nil {
		return false, err
	}

	if obs, err = ctrl.observe(ctx, r, spec, obs.status); err != nil {
		return false, err
	}

	if rejected, err := ctrl.rejected(ctx, r, obs); rejected {
		return obs.waitsOnKernel(), err
	}

	if !obs.enabled() {
		return ctrl.restore(ctx, r, logger, obs)
	}

	if done, err := ctrl.execute(ctx, r, logger, obs); !done {
		return true, err
	}

	return ctrl.finish(ctx, r, obs, true)
}

// rejected publishes a known enforcement loss, an invalid desired policy or a rejected
// transition: nothing applied, reserved or granted changes.
func (ctrl *CPUPartitionController) rejected(ctx context.Context, r controller.Runtime, obs *observation) (bool, error) {
	switch {
	case len(obs.loss) > 0:
		obs.applied.Blocked = obs.loss
	case len(obs.unverified) > 0:
		// A written mask not yet in effect outranks how the desired policy is classified: admission
		// stays closed and the write keeps being verified.
		return true, ctrl.unverified(ctx, r, obs, obs.unverified)
	case obs.desiredErr != nil:
		obs.applied.Blocked = []runtime.CPUPartitionBlock{{Reason: "invalid desired policy: " + obs.desiredErr.Error()}}
	case obs.result.IsBlocked():
		obs.applied.Blocked = nil

		for _, block := range obs.result.Blocked {
			obs.applied.Blocked = append(obs.applied.Blocked, runtime.CPUPartitionBlock{
				Reason:          block.Reason,
				VirtualMachines: block.Consumers,
				CPUs:            cpusString(block.CPUs),
			})
		}
	default:
		return false, nil
	}

	ctrl.barrier = ""
	obs.applied.Error, obs.applied.Waiting = "", ""

	// Blocked keeps admission open under the applied policy; a removed policy keeps it closed.
	obs.applied.Phase = runtime.CPUPartitionPhaseBlocked
	if !obs.enabled() {
		obs.applied.Phase = runtime.CPUPartitionPhaseRestoring
	}

	return true, ctrl.publishStatus(ctx, r, obs)
}

// finish runs once the applied masks equal the desired ones: targets dropped from the policy are
// released, placements are granted and withdrawn, and admission opens. A dropped slice still
// placed or populated is only reported: the machines in it are admitted like any other, which
// is how they leave it.
func (ctrl *CPUPartitionController) finish(ctx context.Context, r controller.Runtime, obs *observation, closed bool) (bool, error) {
	ctrl.barrier = ""

	// Ready, adoption and admission rest on every desired boundary actually being in effect.
	if failures := ctrl.verifyDesired(obs); len(failures) > 0 {
		return true, ctrl.unverified(ctx, r, obs, failures)
	}

	obs.applied.Error, obs.applied.Blocked = "", nil

	pending, err := ctrl.release(ctx, r, obs, closed)
	if err != nil {
		return false, err
	}

	obs.adoptMatching()

	if obs.desired.KubeletReservation == nil {
		// Nothing to stage: the kubelet need not wait for this controller.
		if err = ctrl.publishReservation(ctx, r, nil); err != nil {
			return false, err
		}
	}

	if err = ctrl.admit(ctx, r, obs); err != nil {
		return false, err
	}

	obs.applied.Phase = runtime.CPUPartitionPhaseReady
	obs.applied.Exclusive = exclusiveSlices(obs.spec)

	// Unattributed tasks keep refusing exclusive owners until they exit, which only polling sees.
	return pending || obs.residual, ctrl.publishStatus(ctx, r, obs)
}

// adoptMatching owns every already-matching desired mask from now on, so a later foreign change is
// detected and admission may rely on it; restoring it leaves the value it was found with.
func (obs *observation) adoptMatching() {
	for target, set := range obs.desired.Sets {
		if _, managed := obs.applied.Target(target.Key()); !managed && !obs.restored[target] && obs.actual.Sets[target].Equals(set) {
			obs.applied.SetTarget(runtime.CPUPartitionTargetStatus{Key: target.Key(), Initial: set.String(), LastApplied: set.String(), Intended: set.String()})
		}
	}
}

// unverified keeps admission closed while written masks are not in effect; it is retried by polling.
func (ctrl *CPUPartitionController) unverified(ctx context.Context, r controller.Runtime, obs *observation, failures []string) error {
	obs.applied.Phase = runtime.CPUPartitionPhaseConverging
	if !obs.enabled() {
		obs.applied.Phase = runtime.CPUPartitionPhaseRestoring
	}

	obs.applied.Blocked, obs.applied.Waiting = nil, ""
	obs.applied.Error = strings.Join(failures, "; ")

	return ctrl.publishStatus(ctx, r, obs)
}

// verifyDesired checks the effective set of every target the policy bounds.
func (ctrl *CPUPartitionController) verifyDesired(obs *observation) []string {
	var failures []string

	for _, target := range slices.SortedFunc(maps.Keys(obs.desired.Sets), compareTargets) {
		if obs.restored[target] {
			continue
		}

		if err := ctrl.verifyEffective(target, obs.desired.Sets[target].String()); err != nil {
			failures = append(failures, err.Error())
		}
	}

	return failures
}

func (ctrl *CPUPartitionController) closeAdmission(ctx context.Context, r controller.Runtime, obs *observation) error {
	switch {
	case !obs.enabled():
		obs.applied.Phase = runtime.CPUPartitionPhaseRestoring
	case obs.applied.Phase != runtime.CPUPartitionPhaseConverging:
		obs.applied.Phase = runtime.CPUPartitionPhaseApplying
	}

	obs.applied.Blocked = nil

	return ctrl.publishStatus(ctx, r, obs)
}

// publishStatus writes the status; an unchanged status is not rewritten.
func (ctrl *CPUPartitionController) publishStatus(ctx context.Context, r controller.Runtime, obs *observation) error {
	res, err := safe.WriterModifyWithResult(ctx, r, runtime.NewCPUPartitionStatus(), func(res *runtime.CPUPartitionStatus) error {
		*res.TypedSpec() = obs.applied.DeepCopy()

		return nil
	})
	if err != nil {
		return fmt.Errorf("error publishing CPU partition status: %w", err)
	}

	obs.status = res

	return nil
}

func (ctrl *CPUPartitionController) publishReservation(ctx context.Context, r controller.Runtime, reserved *cpuset.CPUSet) error {
	if err := safe.WriterModify(ctx, r, k8s.NewKubeletCPUReservation(), func(res *k8s.KubeletCPUReservation) error {
		if reserved == nil {
			*res.TypedSpec() = k8s.KubeletCPUReservationSpec{}
		} else {
			*res.TypedSpec() = k8s.KubeletCPUReservationSpec{Managed: true, ReservedCPUs: reserved.String()}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error publishing kubelet CPU reservation: %w", err)
	}

	return nil
}

// cleanupPlacements withdraws every placement not in keep; a claimed one stays tearing down
// until the runtime releases it.
func cleanupPlacements(ctx context.Context, r controller.ReaderWriter, keep map[string]struct{}) error {
	placements, err := safe.ReaderListAll[*hypervisor.VirtualMachineCPUPlacement](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing CPU placements: %w", err)
	}

	for placement := range placements.All() {
		if _, ok := keep[placement.Metadata().ID()]; ok && placement.Metadata().Phase() == resource.PhaseRunning {
			continue
		}

		if err = withdrawPlacement(ctx, r, placement); err != nil {
			return err
		}
	}

	return nil
}

// withdrawPlacement tears the placement down and destroys it once the runtime released it.
func withdrawPlacement(ctx context.Context, r controller.ReaderWriter, placement *hypervisor.VirtualMachineCPUPlacement) error {
	ready, err := r.Teardown(ctx, placement.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error withdrawing CPU placement %q: %w", placement.Metadata().ID(), err)
	}

	if !ready {
		return nil
	}

	if err = r.Destroy(ctx, placement.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error destroying CPU placement %q: %w", placement.Metadata().ID(), err)
	}

	return nil
}

func cpusString(set cpuset.CPUSet) string {
	if set.IsEmpty() {
		return ""
	}

	return set.String()
}
