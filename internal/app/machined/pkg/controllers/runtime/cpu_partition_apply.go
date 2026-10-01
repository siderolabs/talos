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
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// barrierError is a barrier which is not met yet; the plan resumes on the next event or poll.
type barrierError struct{ reason string }

func (e barrierError) Error() string { return e.reason }

// execute runs the plan. A step which fails or hits a barrier leaves the phase converging with
// the reason in the status (Error for a failure, Waiting for a barrier) and is retried by
// polling; a barrier held longer than BarrierTimeout is additionally reported as a stalled
// transition. Nothing is granted, nothing is reverted, the operator decides. Only an error
// publishing the status itself is returned.
func (ctrl *CPUPartitionController) execute(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	rs *reconcileState, steps []cpupartition.Step,
) error {
	var err error

	for _, step := range steps {
		if err = ctrl.executeStep(ctx, r, rs, step); err != nil {
			break
		}
	}

	if err == nil {
		return nil
	}

	rs.applied.Phase = runtime.CPUPartitionPhaseConverging

	var barrier barrierError
	if !errors.As(err, &barrier) {
		rs.applied.Error = err.Error()
		logger.Warn("CPU partition step failed", zap.Error(err))

		return ctrl.publishStatus(ctx, r, rs, rs.applied)
	}

	if ctrl.barrier != barrier.reason {
		ctrl.barrier, ctrl.barrierSince = barrier.reason, time.Now()
	}

	rs.applied.Waiting = barrier.reason
	rs.applied.Error = ""

	if time.Since(ctrl.barrierSince) >= ctrl.BarrierTimeout {
		rs.applied.Error = fmt.Sprintf("transition did not converge within %s: %s", ctrl.BarrierTimeout, barrier.reason)
		logger.Warn("CPU partition transition stalled", zap.String("barrier", barrier.reason))
	} else {
		logger.Debug("CPU partition transition waiting", zap.String("barrier", barrier.reason))
	}

	return ctrl.publishStatus(ctx, r, rs, rs.applied)
}

func (ctrl *CPUPartitionController) executeStep(ctx context.Context, r controller.Runtime, rs *reconcileState, step cpupartition.Step) error {
	switch step.Kind {
	case cpupartition.StepPublishKubeletReservation:
		return ctrl.publishReservation(ctx, r, step.Reservation)

	case cpupartition.StepAwaitKubepodsRelease:
		leaves, err := ctrl.FS.LeafEffective(constants.CgroupKubepods)
		if err != nil {
			return fmt.Errorf("error reading kubepods leaves: %w", err)
		}

		for _, leaf := range slices.Sorted(maps.Keys(leaves)) {
			if overlap := leaves[leaf].Intersection(step.CPUs); !overlap.IsEmpty() {
				return barrierError{fmt.Sprintf("kubepods leaf %q still runs on CPUs %s", leaf, overlap)}
			}
		}

		return nil

	case cpupartition.StepSetCPUs:
		return ctrl.writeTarget(ctx, r, rs, step.Target, step.CPUs.String())

	default:
		return fmt.Errorf("unknown step %v", step.Kind)
	}
}

// writeTarget records the intent, writes the mask, verifies the effective set and records the
// result; the intent is published first so a crash between write and publication is recoverable.
// A target whose cgroup does not exist yet (kubepods before the kubelet created it) is a barrier:
// the plan does not complete, and nothing is admitted on it, until the cap is verified.
func (ctrl *CPUPartitionController) writeTarget(ctx context.Context, r controller.Runtime, rs *reconcileState, target cpupartition.Target, cpus string) error {
	path := target.CgroupPath()

	if err := ctrl.ensureTarget(target); err != nil {
		return err
	}

	entry, managed := rs.applied.Target(target.Key())
	if !managed {
		initial, err := ctrl.FS.CPUs(path)
		if err != nil {
			return err
		}

		entry = runtime.CPUPartitionTargetStatus{Key: target.Key(), Initial: initial, LastApplied: initial}
	}

	if entry.LastApplied == cpus {
		return nil
	}

	entry.Intended = cpus
	rs.applied.SetTarget(entry)

	if err := ctrl.publishStatus(ctx, r, rs, rs.applied); err != nil {
		return err
	}

	if err := ctrl.FS.SetCPUs(path, cpus); err != nil {
		return fmt.Errorf("error writing %s: %w", target, err)
	}

	if err := ctrl.verifyEffective(target, cpus); err != nil {
		return err
	}

	entry.LastApplied = cpus
	entry.Intended = cpus
	rs.applied.SetTarget(entry)

	return ctrl.publishStatus(ctx, r, rs, rs.applied)
}

// verifyEffective checks that the kernel exposes exactly the written mask; an empty mask
// (inheritance restored) has nothing to verify against.
func (ctrl *CPUPartitionController) verifyEffective(target cpupartition.Target, cpus string) error {
	if cpus == "" {
		return nil
	}

	effective, err := ctrl.FS.Effective(target.CgroupPath())
	if err != nil {
		return err
	}

	want, err := cpuset.Parse(cpus)
	if err != nil {
		return fmt.Errorf("%s: invalid CPU list %q: %w", target, cpus, err)
	}

	if !effective.Equals(want) {
		return fmt.Errorf("%s: effective CPUs %q differ from written %q", target, effective, cpus)
	}

	return nil
}

// ensureTarget makes the target's cgroup exist; kubepods is the kubelet's and is a barrier until
// the kubelet has created it.
func (ctrl *CPUPartitionController) ensureTarget(target cpupartition.Target) error {
	path := target.CgroupPath()

	if target != cpupartition.Root(config.CPUPartitionRootKubepods) {
		if err := ctrl.FS.Ensure(path); err != nil {
			return fmt.Errorf("error creating %s: %w", target, err)
		}

		return nil
	}

	exists, err := ctrl.FS.Exists(path)
	if err != nil {
		return err
	}

	if !exists {
		return barrierError{"kubepods cgroup has not been created by the kubelet yet"}
	}

	return nil
}

// recoverApplied re-reads every managed cgroup: a write whose Intended differs from LastApplied
// is confirmed or forgotten from the kernel value, and any other mismatch is foreign drift.
func (ctrl *CPUPartitionController) recoverApplied(applied *runtime.CPUPartitionStatusSpec) ([]cpupartition.Target, error) {
	var drift []cpupartition.Target

	for _, entry := range slices.Clone(applied.Targets) {
		target, ok := cpupartition.ParseKey(entry.Key)
		if !ok {
			continue
		}

		drifted, err := ctrl.recoverTarget(applied, target, entry)
		if err != nil {
			return nil, err
		}

		if drifted {
			drift = append(drift, target)
		}
	}

	return drift, nil
}

// recoverTarget reconciles one managed cgroup with the kernel; it reports foreign drift. A
// pending intent is confirmed when the kernel holds it, forgotten when the kernel still holds
// the last applied value, and any third value is drift.
func (ctrl *CPUPartitionController) recoverTarget(applied *runtime.CPUPartitionStatusSpec, target cpupartition.Target, entry runtime.CPUPartitionTargetStatus) (bool, error) {
	exists, err := ctrl.FS.Exists(target.CgroupPath())
	if err != nil {
		return false, err
	}

	if !exists {
		// kubepods is created by the kubelet: it may legitimately be gone across a restart.
		if target == cpupartition.Root(config.CPUPartitionRootKubepods) {
			applied.DeleteTarget(entry.Key)

			return false, nil
		}

		return true, nil
	}

	current, err := ctrl.FS.CPUs(target.CgroupPath())
	if err != nil {
		return false, err
	}

	if entry.Intended != "" && entry.Intended != entry.LastApplied {
		if current == entry.Intended {
			entry.LastApplied = entry.Intended
		}

		entry.Intended = entry.LastApplied
		applied.SetTarget(entry)
	}

	// After a pending intent is resolved, a kernel value matching neither the intent nor the last
	// applied value is foreign drift like any other mismatch.
	return current != entry.LastApplied, nil
}

// restore puts every managed cgroup back to its initial value when it still holds what was last
// applied, removes the slice partitions, and finally drops the status: the admission guard is
// the last thing to go. Every placement must have been released first; the operator stops the
// machines, this controller never does.
func (ctrl *CPUPartitionController) restore(ctx context.Context, r controller.Runtime, rs *reconcileState) (bool, error) {
	rs.applied.Phase = runtime.CPUPartitionPhaseRestoring

	if len(rs.placements) > 0 {
		return false, errors.Join(ctrl.publishStatus(ctx, r, rs, rs.applied), ctrl.cleanupPlacements(ctx, r, nil))
	}

	// Children before the root, so the root never shrinks under a wider child.
	entries := slices.SortedFunc(slices.Values(rs.applied.Targets), func(a, b runtime.CPUPartitionTargetStatus) int {
		ta, _ := cpupartition.ParseKey(a.Key)
		tb, _ := cpupartition.ParseKey(b.Key)

		return -cmpKind(ta, tb)
	})

	for _, entry := range entries {
		pending, err := ctrl.restoreTarget(ctx, r, rs, entry)
		if err != nil || pending {
			return pending, err
		}
	}

	if rs.status != nil {
		if err := r.Destroy(ctx, rs.status.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return false, fmt.Errorf("error removing CPU partition status: %w", err)
		}
	}

	return false, nil
}

// restoreTarget restores one cgroup: a root gets its initial mask back when its mask is still
// what was last applied, a slice partition is removed once empty; pending=true while it has tasks.
func (ctrl *CPUPartitionController) restoreTarget(ctx context.Context, r controller.Runtime, rs *reconcileState, entry runtime.CPUPartitionTargetStatus) (bool, error) {
	target, _ := cpupartition.ParseKey(entry.Key)
	path := target.CgroupPath()

	exists, err := ctrl.FS.Exists(path)
	if err != nil {
		return false, err
	}

	if exists {
		current, err := ctrl.FS.CPUs(path)
		if err != nil {
			return false, err
		}

		if current != entry.LastApplied {
			rs.applied.Error = fmt.Sprintf("%s: not restored, its mask %q was changed outside Talos", target, current)

			return false, ctrl.publishStatus(ctx, r, rs, rs.applied)
		}

		pending, err := ctrl.restoreExisting(ctx, r, rs, target, entry)
		if err != nil || pending {
			return pending, err
		}
	}

	rs.applied.DeleteTarget(entry.Key)

	return false, ctrl.publishStatus(ctx, r, rs, rs.applied)
}

// restoreExisting writes a root's restored mask, or removes an empty slice partition.
func (ctrl *CPUPartitionController) restoreExisting(ctx context.Context, r controller.Runtime, rs *reconcileState, target cpupartition.Target, entry runtime.CPUPartitionTargetStatus) (bool, error) {
	path := target.CgroupPath()

	if target.Kind == cpupartition.KindRoot {
		restored, err := ctrl.restoredMask(target, entry.Initial)
		if err != nil {
			return false, err
		}

		return false, ctrl.writeTarget(ctx, r, rs, target, restored)
	}

	populated, err := ctrl.FS.Populated(path)
	if err != nil {
		return false, err
	}

	if populated {
		rs.applied.Error = fmt.Sprintf("%s: still has tasks", target)

		return true, ctrl.publishStatus(ctx, r, rs, rs.applied)
	}

	if err = ctrl.FS.Remove(path); err != nil {
		return false, fmt.Errorf("error removing %s: %w", target, err)
	}

	return false, nil
}

// restoredMask is the value written back to a root on removal. An initial empty mask (inherit)
// cannot be written back to a cgroup with tasks: cgroup v2 refuses to empty such a cpuset
// (ENOSPC, observed live). The parent's effective set is what an empty mask resolves to and is
// accepted, so it is written instead; the root is then bounded by its parent exactly as before.
func (ctrl *CPUPartitionController) restoredMask(target cpupartition.Target, initial string) (string, error) {
	if initial != "" {
		return initial, nil
	}

	populated, err := ctrl.FS.Populated(target.CgroupPath())
	if err != nil || !populated {
		return "", err
	}

	parent, err := ctrl.FS.Effective(parentPath(target.CgroupPath()))
	if err != nil {
		return "", fmt.Errorf("error reading the parent of %s: %w", target, err)
	}

	return parent.String(), nil
}

// parentPath returns the cgroup path of the parent; the root's parent is the cgroup root ("").
func parentPath(path string) string {
	parent, _, found := strings.CutLast(path, "/")
	if !found {
		return ""
	}

	return parent
}

func cmpKind(a, b cpupartition.Target) int {
	if a.Kind == b.Kind {
		return 0
	}

	if a.Kind == cpupartition.KindRoot {
		return -1
	}

	return 1
}
