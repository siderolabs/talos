// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// barrierError is a condition outside Talos which the transition waits for.
type barrierError struct{ reason string }

func (e barrierError) Error() string { return e.reason }

// execute runs the planned steps. It reports false while a step waits on a barrier or failed:
// the phase stays closed, the reason is in the status, and the pass is retried by polling with
// the plan recomputed from the kernel. Nothing is reverted.
func (ctrl *CPUPartitionController) execute(ctx context.Context, r controller.Runtime, logger *zap.Logger, obs *observation) (bool, error) {
	var err error

	for _, step := range obs.result.Steps {
		if err = ctrl.executeStep(ctx, r, obs, step); err != nil {
			break
		}
	}

	if err == nil {
		return true, nil
	}

	if obs.enabled() {
		obs.applied.Phase = runtime.CPUPartitionPhaseConverging
	}

	obs.applied.Error, obs.applied.Waiting = "", ""

	var barrier barrierError
	if !errors.As(err, &barrier) {
		ctrl.barrier = ""
		obs.applied.Error = err.Error()

		logger.Warn("CPU partition step failed", zap.Error(err))

		return false, ctrl.publishStatus(ctx, r, obs)
	}

	if ctrl.barrier != barrier.reason {
		ctrl.barrier, ctrl.barrierSince = barrier.reason, time.Now()
	}

	obs.applied.Waiting = barrier.reason

	if time.Since(ctrl.barrierSince) >= ctrl.BarrierTimeout {
		obs.applied.Error = fmt.Sprintf("transition did not converge within %s: %s", ctrl.BarrierTimeout, barrier.reason)

		logger.Warn("CPU partition transition stalled", zap.String("barrier", barrier.reason))
	}

	return false, ctrl.publishStatus(ctx, r, obs)
}

func (ctrl *CPUPartitionController) executeStep(ctx context.Context, r controller.Runtime, obs *observation, step cpupartition.Step) error {
	switch step.Kind {
	case cpupartition.StepSetCPUs:
		return ctrl.writeTarget(ctx, r, obs, step.Target, step.CPUs.String())
	case cpupartition.StepPublishKubeletReservation:
		return ctrl.publishReservation(ctx, r, step.Reservation)
	case cpupartition.StepAwaitKubepodsRelease:
		// Also on a retry which no longer publishes: publication is not consumption.
		if err := kubeletReadBack(ctx, r, obs.desired.KubeletReservation); err != nil {
			return err
		}

		return ctrl.awaitKubepodsRelease(step.CPUs)
	default:
		return fmt.Errorf("unknown CPU partition step %v", step.Kind)
	}
}

// kubeletReadBack waits until the rendered kubelet configuration carries the reservation (or,
// unmanaged, none).
func kubeletReadBack(ctx context.Context, r controller.Runtime, reserved *cpuset.CPUSet) error {
	spec, err := safe.ReaderGetByID[*k8s.KubeletSpec](ctx, r, k8s.KubeletID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return barrierError{"kubelet configuration has not been rendered yet"}
		}

		return fmt.Errorf("error getting kubelet spec: %w", err)
	}

	value, isString := spec.TypedSpec().Config["reservedSystemCPUs"].(string)
	if raw, present := spec.TypedSpec().Config["reservedSystemCPUs"]; present && !isString {
		return fmt.Errorf("kubelet configuration has an invalid reservedSystemCPUs %v", raw)
	}

	if reserved == nil {
		if value != "" {
			return barrierError{fmt.Sprintf("kubelet configuration still reserves CPUs %q", value)}
		}

		return nil
	}

	if value == "" {
		return barrierError{fmt.Sprintf("kubelet configuration does not reserve CPUs %q yet", reserved)}
	}

	configured, err := cpuset.Parse(value)
	if err != nil {
		return fmt.Errorf("kubelet configuration has an invalid reservedSystemCPUs %q: %w", value, err)
	}

	if !configured.Equals(*reserved) {
		return barrierError{fmt.Sprintf("kubelet configuration reserves CPUs %q, waiting for %q", configured, reserved)}
	}

	return nil
}

func (ctrl *CPUPartitionController) awaitKubepodsRelease(removed cpuset.CPUSet) error {
	leaves, err := ctrl.FS.LeafEffective(constants.CgroupKubepods)
	if err != nil {
		return fmt.Errorf("error reading kubepods leaves: %w", err)
	}

	for _, leaf := range slices.Sorted(maps.Keys(leaves)) {
		if overlap := leaves[leaf].Intersection(removed); !overlap.IsEmpty() {
			return barrierError{fmt.Sprintf("kubepods leaf %q still runs on CPUs %s", leaf, overlap)}
		}
	}

	return nil
}

// writeTarget writes one mask. The intent, and on first management the original mask, are
// published before the cgroup is created or written, so an interrupted write is recovered from
// the status and the kernel alone. Only a verified effective set completes the write.
func (ctrl *CPUPartitionController) writeTarget(ctx context.Context, r controller.Runtime, obs *observation, target cpupartition.Target, cpus string) error {
	path := target.CgroupPath()

	exists, err := ctrl.FS.Exists(path)
	if err != nil {
		return fmt.Errorf("error checking %s: %w", target, err)
	}

	if !exists && target == kubepodsTarget {
		return barrierError{"kubepods cgroup has not been created by the kubelet yet"}
	}

	entry, err := ctrl.targetEntry(obs, target)
	if err != nil {
		return err
	}

	entry.Intended = cpus
	obs.applied.SetTarget(entry)

	if err = ctrl.publishStatus(ctx, r, obs); err != nil {
		return err
	}

	if !exists {
		if err = ctrl.FS.Ensure(path); err != nil {
			return fmt.Errorf("error creating %s: %w", target, err)
		}
	}

	if err = ctrl.FS.SetCPUs(path, cpus); err != nil {
		return fmt.Errorf("error writing %s: %w", target, err)
	}

	if err = ctrl.verifyEffective(target, cpus); err != nil {
		return err
	}

	entry.LastApplied = cpus
	obs.applied.SetTarget(entry)

	return ctrl.publishStatus(ctx, r, obs)
}

// targetEntry returns the status of a target; on first management it records the original mask.
func (ctrl *CPUPartitionController) targetEntry(obs *observation, target cpupartition.Target) (runtime.CPUPartitionTargetStatus, error) {
	if entry, managed := obs.applied.Target(target.Key()); managed {
		return entry, nil
	}

	initial, _, err := ctrl.readMask(target)

	return runtime.CPUPartitionTargetStatus{Key: target.Key(), Initial: initial, LastApplied: initial}, err
}

// verifyEffective checks the kernel exposes the written mask; an empty mask inherits.
func (ctrl *CPUPartitionController) verifyEffective(target cpupartition.Target, cpus string) error {
	if cpus == "" {
		return nil
	}

	want, err := cpuset.Parse(cpus)
	if err != nil {
		return fmt.Errorf("%s: invalid CPU list %q: %w", target, cpus, err)
	}

	effective, err := ctrl.FS.Effective(target.CgroupPath())
	if err != nil {
		return fmt.Errorf("error reading the effective CPUs of %s: %w", target, err)
	}

	if !effective.Equals(want) {
		return fmt.Errorf("%s: effective CPUs %q differ from the written %q", target, effective, cpus)
	}

	return nil
}

// release gives up the targets the policy no longer names once the plan has put their
// restoration masks in place; with admission open it changes nothing and a releasable target
// only asks for another pass. It reports true when only the kernel can tell when to retry.
func (ctrl *CPUPartitionController) release(ctx context.Context, r controller.Runtime, obs *observation, closed bool) (bool, error) {
	var (
		waiting []string
		pending bool
	)

	// Slices go before the virtual machine root they are bounded by.
	entries := slices.SortedFunc(slices.Values(obs.applied.Targets), func(a, b runtime.CPUPartitionTargetStatus) int {
		ta, _ := cpupartition.ParseKey(a.Key) //nolint:errcheck // validated during recovery
		tb, _ := cpupartition.ParseKey(b.Key) //nolint:errcheck // validated during recovery

		return cmp.Or(-compareTargets(ta, tb), cmp.Compare(a.Key, b.Key))
	})

	for _, entry := range entries {
		target, _ := cpupartition.ParseKey(entry.Key) //nolint:errcheck // validated during recovery

		if _, desired := obs.desired.Sets[target]; desired && !obs.restored[target] {
			continue
		}

		reason, poll, err := ctrl.releaseTarget(ctx, r, obs, target, entry, closed)
		if err != nil {
			return false, err
		}

		if reason != "" {
			waiting = append(waiting, reason)
			pending = pending || poll
		}
	}

	obs.applied.Waiting = strings.Join(waiting, "; ")

	if strings.Contains(obs.applied.Waiting, "restoration failed") {
		obs.applied.Error = obs.applied.Waiting
	}

	return pending, nil
}

// releaseTarget releases one target; it returns why it cannot yet, and whether only the kernel
// can tell when it can.
//
// A slice partition is removed once no placement names it and its cgroup has no tasks; the
// removal intent (an empty Intended, never a planned mask) is published first. A fixed root with
// an inherited original is first planned back to its parent's effective set and then gets the
// empty mask back, announced like any write; the target stays owned until that write succeeded.
func (ctrl *CPUPartitionController) releaseTarget(ctx context.Context, r controller.Runtime, obs *observation,
	target cpupartition.Target, entry runtime.CPUPartitionTargetStatus, closed bool,
) (string, bool, error) {
	if holder := obs.placedFor(target); target.Kind != cpupartition.KindRoot && holder != "" {
		return fmt.Sprintf("%s: waiting for the CPU placement of %s to be released", entry.Key, holder), false, nil
	}

	populated, err := ctrl.populated(target)
	if err != nil {
		return "", false, err
	}

	switch {
	case target.Kind != cpupartition.KindRoot && populated:
		return entry.Key + ": waiting for its tasks to exit", true, nil
	case !closed:
		return entry.Key + ": released", true, nil
	}

	// A refused write or removal keeps the target owned and its restoration pending.
	if err = ctrl.giveUp(ctx, r, obs, target, entry); err != nil {
		return fmt.Sprintf("%s: restoration failed: %s", entry.Key, err), true, nil
	}

	return "", false, nil
}

// giveUp removes a slice, or puts a root with an inherited original back on inheritance, and only
// then stops managing the target.
func (ctrl *CPUPartitionController) giveUp(ctx context.Context, r controller.Runtime, obs *observation,
	target cpupartition.Target, entry runtime.CPUPartitionTargetStatus,
) error {
	var err error

	switch {
	case target.Kind != cpupartition.KindRoot:
		err = ctrl.removeSlice(ctx, r, obs, target, entry)
	case entry.Initial == "" && entry.LastApplied != "":
		err = ctrl.writeTarget(ctx, r, obs, target, "")
	}

	if err != nil {
		return err
	}

	obs.applied.DeleteTarget(entry.Key)

	return ctrl.publishStatus(ctx, r, obs)
}

// removeSlice announces the removal (an empty intent) before removing the cgroup.
func (ctrl *CPUPartitionController) removeSlice(ctx context.Context, r controller.Runtime, obs *observation,
	target cpupartition.Target, entry runtime.CPUPartitionTargetStatus,
) error {
	entry.Intended = ""
	obs.applied.SetTarget(entry)

	if err := ctrl.publishStatus(ctx, r, obs); err != nil {
		return err
	}

	if err := ctrl.FS.Remove(target.CgroupPath()); err != nil {
		return fmt.Errorf("error removing %s: %w", target, err)
	}

	return nil
}

// restore completes the removal of the policy with admission closed: placements are withdrawn and
// nothing is restored until the runtime has released every one of them; then the original masks
// are planned back like any transition, released targets are given up and finally the status,
// the admission guard, goes.
func (ctrl *CPUPartitionController) restore(ctx context.Context, r controller.Runtime, logger *zap.Logger, obs *observation) (bool, error) {
	if err := cleanupPlacements(ctx, r, nil); err != nil {
		return false, err
	}

	if held := slices.Sorted(maps.Keys(obs.placements)); len(held) > 0 {
		obs.applied.Waiting = fmt.Sprintf("waiting for the CPU placements of %s to be released", strings.Join(held, ", "))

		return false, ctrl.publishStatus(ctx, r, obs)
	}

	if done, err := ctrl.execute(ctx, r, logger, obs); !done {
		return true, err
	}

	obs.applied.Error = ""

	pending, err := ctrl.release(ctx, r, obs, true)
	if err != nil {
		return false, err
	}

	if len(obs.applied.Targets) > 0 {
		return pending || obs.residual, ctrl.publishStatus(ctx, r, obs)
	}

	return false, ctrl.dropStatus(ctx, r, obs)
}

// dropStatus ends the policy: the reservation is unmanaged, and the status, the admission guard, goes last.
func (ctrl *CPUPartitionController) dropStatus(ctx context.Context, r controller.Runtime, obs *observation) error {
	if err := ctrl.publishReservation(ctx, r, nil); err != nil {
		return err
	}

	ctrl.barrier = ""

	if obs.status == nil {
		return nil
	}

	if err := r.Destroy(ctx, obs.status.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error removing CPU partition status: %w", err)
	}

	return nil
}
