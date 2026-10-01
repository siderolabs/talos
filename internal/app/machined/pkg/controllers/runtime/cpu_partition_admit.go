// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// admit grants a placement to every released virtual machine which passes the applied policy,
// in the partition it gives it; a granted placement is only ever replaced through teardown.
// Machines refused a placement are listed in the status; unrelated machines are unaffected.
func (ctrl *CPUPartitionController) admit(ctx context.Context, r controller.Runtime, rs *reconcileState) error {
	rs.applied.AdmissionErrors = nil

	exclusiveOwners := map[string][]string{}

	for _, consumer := range rs.consumers {
		if consumer.Desired.Kind == cpupartition.KindSlice && slices.Contains(rs.applied.Exclusive, consumer.Desired.Name) {
			exclusiveOwners[consumer.Desired.Name] = append(exclusiveOwners[consumer.Desired.Name], consumer.Name)
		}
	}

	keep := map[string]struct{}{}

	var errs error

	for _, consumer := range rs.consumers {
		switch admissionDecision(rs, consumer, exclusiveOwners) {
		case admitKeep:
			keep[consumer.Name] = struct{}{}
		case admitGrant:
			keep[consumer.Name] = struct{}{}

			errs = errors.Join(errs, ctrl.grant(ctx, r, rs, consumer))
		case admitDrop:
		}
	}

	return errors.Join(errs, ctrl.cleanupPlacements(ctx, r, keep))
}

type admission int

const (
	// admitDrop: no placement (none wanted, refused, or the old grant must go first).
	admitDrop admission = iota
	// admitKeep: an occupying machine keeps its placement whatever the desired policy says.
	admitKeep
	// admitGrant: a released machine passing the policy gets its placement.
	admitGrant
)

// admissionDecision decides one consumer; a refusal is recorded in the status.
func admissionDecision(rs *reconcileState, consumer cpupartition.Consumer, exclusiveOwners map[string][]string) admission {
	switch {
	case consumer.Orphaned && consumer.State == cpupartition.ConsumerReleased:
		// Its virtual machine is gone and it released: the placement goes.
		return admitDrop
	case consumer.State != cpupartition.ConsumerReleased:
		return admitKeep
	case consumer.Desired.Kind == cpupartition.KindRoot:
		// No virtual machine partitioning: no placement.
		return admitDrop
	}

	if reason := admissionError(&rs.applied, consumer, exclusiveOwners); reason != "" {
		rs.applied.AdmissionErrors = append(rs.applied.AdmissionErrors, runtime.CPUPartitionBlock{Reason: reason, VirtualMachines: []string{consumer.Name}})

		return admitDrop
	}

	if existing, ok := rs.placements[consumer.Name]; ok && placementKey(existing.TypedSpec()) != consumer.Desired.Key() {
		// Retarget of a released machine: the old grant goes first, the new one next time.
		return admitDrop
	}

	return admitGrant
}

func (ctrl *CPUPartitionController) grant(ctx context.Context, r controller.Runtime, rs *reconcileState, consumer cpupartition.Consumer) error {
	target := consumer.Desired

	err := safe.WriterModify(ctx, r, hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, consumer.Name),
		func(res *hypervisor.VirtualMachineCPUPlacement) error {
			*res.TypedSpec() = hypervisor.VirtualMachineCPUPlacementSpec{
				Partition: target.Partition(),
				Slice:     target.Name,
				Exclusive: target.Kind == cpupartition.KindSlice && slices.Contains(rs.applied.Exclusive, target.Name),
			}

			return nil
		})
	if err != nil {
		return fmt.Errorf("error writing placement %q: %w", consumer.Name, err)
	}

	return nil
}

// admissionError checks one released machine against the applied policy.
func admissionError(applied *runtime.CPUPartitionStatusSpec, consumer cpupartition.Consumer, exclusiveOwners map[string][]string) string {
	target := consumer.Desired

	if target.Kind == cpupartition.KindSlice && slices.Contains(applied.Exclusive, target.Name) && len(exclusiveOwners[target.Name]) > 1 {
		return fmt.Sprintf("exclusive slice %q is selected by more than one virtual machine: %v", target.Name, exclusiveOwners[target.Name])
	}

	// An undeclared target has no entry and therefore no CPUs.
	entry, _ := applied.Target(target.Key())

	allocation, err := cpuset.Parse(entry.LastApplied)
	if err != nil || allocation.IsEmpty() {
		return fmt.Sprintf("%s has no applied CPUs", target)
	}

	if !consumer.Pins.IsSubsetOf(allocation) {
		return fmt.Sprintf("host CPU pins %q fall outside %s (%q)", consumer.Pins, target, allocation)
	}

	return ""
}
