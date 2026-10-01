// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// occupancy is the evidence of where virtual machines actually are.
type occupancy struct {
	// placements by virtual machine name.
	placements map[string]*hypervisor.VirtualMachineCPUPlacement
	// observed domains (libvirt still reports them), by name.
	observed map[string]struct{}
	// claimed domain specs (the runtime holds them: running or a start in flight), by name.
	claimed map[string]struct{}
}

// observeConsumers builds the classifier's view of every virtual machine: where it actually runs,
// where it wants to run, its pins, and whether it still occupies.
//
// A running machine without a placement (started before the policy managed virtual machines)
// occupies the virtual machine root; introducing slices would change its placement, which is
// a blocked transition until the operator stops it. Domains and claims whose virtual machine
// spec vanished still occupy until released.
func (ctrl *CPUPartitionController) observeConsumers(ctx context.Context, r controller.Runtime,
	spec *runtime.CPUPartitionSpec,
) ([]cpupartition.Consumer, map[string]*hypervisor.VirtualMachineCPUPlacement, error) {
	occ, err := ctrl.readOccupancy(ctx, r)
	if err != nil {
		return nil, nil, err
	}

	vms, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return nil, nil, fmt.Errorf("error listing virtual machine specs: %w", err)
	}

	seen := map[string]struct{}{}

	var consumers []cpupartition.Consumer

	for vm := range vms.All() {
		name := vm.Metadata().ID()
		seen[name] = struct{}{}

		consumer, err := ctrl.consumerOf(occ, name, desiredTarget(spec, vm.TypedSpec().CPU.Slice), false)
		if err != nil {
			return nil, nil, err
		}

		consumer.Pins = pinsOf(vm.TypedSpec().CPU)
		consumers = append(consumers, consumer)
	}

	// Machines whose spec is gone: a placement, a claim or a domain still occupies until released.
	for _, name := range slices.Sorted(occ.names()) {
		if _, ok := seen[name]; ok {
			continue
		}

		consumer, err := ctrl.consumerOf(occ, name, cpupartition.Target{}, true)
		if err != nil {
			return nil, nil, err
		}

		if consumer.State == cpupartition.ConsumerReleased && occ.placements[name] == nil {
			continue
		}

		consumer.Desired = consumer.Applied
		consumers = append(consumers, consumer)
	}

	return consumers, occ.placements, nil
}

func (ctrl *CPUPartitionController) readOccupancy(ctx context.Context, r controller.Runtime) (*occupancy, error) {
	occ := &occupancy{
		placements: map[string]*hypervisor.VirtualMachineCPUPlacement{},
		observed:   map[string]struct{}{},
		claimed:    map[string]struct{}{},
	}

	placements, err := safe.ReaderListAll[*hypervisor.VirtualMachineCPUPlacement](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing placements: %w", err)
	}

	for placement := range placements.All() {
		occ.placements[placement.Metadata().ID()] = placement
	}

	domainStatuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainStatus](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing domain statuses: %w", err)
	}

	for status := range domainStatuses.All() {
		occ.observed[status.Metadata().ID()] = struct{}{}
	}

	domainSpecs, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing domain specs: %w", err)
	}

	for domainSpec := range domainSpecs.All() {
		if domainSpec.Metadata().Finalizers().Has(virtualMachineControllerName) {
			occ.claimed[domainSpec.Metadata().ID()] = struct{}{}
		}
	}

	return occ, nil
}

func (occ *occupancy) names() func(yield func(string) bool) {
	return func(yield func(string) bool) {
		emitted := map[string]struct{}{}

		for _, names := range []map[string]struct{}{occ.observed, occ.claimed} {
			for name := range names {
				if _, done := emitted[name]; done {
					continue
				}

				emitted[name] = struct{}{}

				if !yield(name) {
					return
				}
			}
		}

		for name := range occ.placements {
			if _, done := emitted[name]; !done && !yield(name) {
				return
			}
		}
	}
}

// consumerOf decides where a machine is and whether it occupies: its placement's partition when
// it has one, the virtual machine root otherwise.
func (ctrl *CPUPartitionController) consumerOf(occ *occupancy, name string, desired cpupartition.Target, orphaned bool) (cpupartition.Consumer, error) {
	consumer := cpupartition.Consumer{Name: name, Desired: desired, Orphaned: orphaned}

	placement := occ.placements[name]

	switch {
	case placement != nil:
		consumer.Applied, _ = cpupartition.ParseKey(placementKey(placement.TypedSpec()))
	default:
		consumer.Applied = cpupartition.Root(config.CPUPartitionRootVirtualMachines)
	}

	var err error

	consumer.State, err = ctrl.consumerState(occ, name, placement)

	return consumer, err
}

// consumerState decides whether a machine still occupies its partition.
//
// A claimed placement is a start in flight or a running domain in that partition. A claimed
// domain spec is a start in flight or a running domain: the runtime never holds a domain claim
// while merely waiting for a placement (it claims the placement first), and releases a domain
// claim taken under a policy read which turned out stale before any start. An observed domain
// occupies until it is gone, and an exclusive slice's cgroup until it is unpopulated.
func (ctrl *CPUPartitionController) consumerState(occ *occupancy, name string, placement *hypervisor.VirtualMachineCPUPlacement) (cpupartition.ConsumerState, error) {
	if placement != nil && placement.Metadata().Finalizers().Has(virtualMachineControllerName) {
		return cpupartition.ConsumerRunning, nil
	}

	if _, ok := occ.claimed[name]; ok {
		return cpupartition.ConsumerRunning, nil
	}

	if _, ok := occ.observed[name]; ok {
		return cpupartition.ConsumerUnreleased, nil
	}

	if placement == nil || !placement.TypedSpec().Exclusive {
		return cpupartition.ConsumerReleased, nil
	}

	populated, err := ctrl.partitionPopulated(placement)
	if err != nil || populated {
		return cpupartition.ConsumerUnreleased, err
	}

	return cpupartition.ConsumerReleased, nil
}

// partitionPopulated reports whether the placement's slice cgroup still has tasks; a missing
// cgroup has none.
func (ctrl *CPUPartitionController) partitionPopulated(placement *hypervisor.VirtualMachineCPUPlacement) (bool, error) {
	target, _ := cpupartition.ParseKey(placementKey(placement.TypedSpec()))

	exists, err := ctrl.FS.Exists(target.CgroupPath())
	if err != nil || !exists {
		return false, err
	}

	return ctrl.FS.Populated(target.CgroupPath())
}
