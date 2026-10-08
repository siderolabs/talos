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

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// admission is the placement change one pass would make under the desired policy.
type admission struct {
	grants   map[string]hypervisor.VirtualMachineCPUPlacementSpec
	withdraw []string
	errors   []runtime.CPUPartitionBlock
}

// planAdmission decides every machine's placement.
//
// A machine which may still occupy its partition keeps whatever placement it has: only the virtual
// machine runtime ends its occupancy, by removing the domain and releasing its claims. A released
// machine gets the placement of the partition its specification selects, once the old placement
// is gone (release before reuse), unless the selection is refused. A machine without
// specification, or without a partition to be placed in, loses its placement.
func planAdmission(obs *observation) admission {
	adm := admission{grants: map[string]hypervisor.VirtualMachineCPUPlacementSpec{}}

	for _, name := range slices.Sorted(maps.Keys(obs.vms)) {
		if vm := obs.vms[name]; !vm.occupies() {
			adm.decide(obs, name, vm)
		}
	}

	return adm
}

// decide places one released machine.
func (adm *admission) decide(obs *observation, name string, vm *vmObservation) {
	grant, reason, wanted := placementFor(obs, name, vm)

	if reason != "" {
		adm.errors = append(adm.errors, runtime.CPUPartitionBlock{Reason: reason, VirtualMachines: []string{name}})
	}

	switch {
	case vm.placement != nil && (!wanted || !placementRunning(vm.placement) || *vm.placement.TypedSpec() != grant):
		// A granted placement is never modified: it goes first, the new one is granted next.
		adm.withdraw = append(adm.withdraw, name)
	case vm.placement == nil && wanted:
		adm.grants[name] = grant
	}
}

// placementFor returns the placement a released machine is admitted with, or why it is refused.
func placementFor(obs *observation, name string, vm *vmObservation) (hypervisor.VirtualMachineCPUPlacementSpec, string, bool) {
	if vm.spec == nil || !desiredManagesVirtualMachines(obs.spec) {
		return hypervisor.VirtualMachineCPUPlacementSpec{}, "", false
	}

	target := cpupartition.Shared
	if vm.spec.TypedSpec().CPU.Slice != "" {
		target = cpupartition.Slice(vm.spec.TypedSpec().CPU.Slice)
	}

	if reason := admissionError(obs, name, target); reason != "" {
		return hypervisor.VirtualMachineCPUPlacementSpec{}, reason, false
	}

	return hypervisor.VirtualMachineCPUPlacementSpec{
		Partition: target.Partition(),
		Slice:     target.Name,
		Exclusive: target.Kind == cpupartition.KindSlice && obs.desired.Exclusive[target.Name],
	}, "", true
}

// admissionError checks one released machine against the desired, applied policy.
func admissionError(obs *observation, name string, target cpupartition.Target) string {
	allocation, ok := obs.desired.Sets[target]
	if !ok {
		return fmt.Sprintf("%s is not declared by the CPU partition policy", target)
	}

	pins, err := specPins(obs.vms[name].spec.TypedSpec().CPU)
	if err != nil {
		return fmt.Sprintf("invalid host CPU pins: %s", err)
	}

	if !pins.IsSubsetOf(allocation) {
		return fmt.Sprintf("host CPU pins %q fall outside %s (%q)", pins, target, allocation)
	}

	if target.Kind != cpupartition.KindSlice || !obs.desired.Exclusive[target.Name] {
		return ""
	}

	return exclusiveConflict(obs, name, target)
}

// exclusiveConflict refuses an exclusive slice still placed for another machine, or selected by
// several machines none of which holds it yet.
func exclusiveConflict(obs *observation, name string, target cpupartition.Target) string {
	var holders, selectors []string

	for _, other := range slices.Sorted(maps.Keys(obs.vms)) {
		switch vm := obs.vms[other]; {
		case other == name:
		case vm.holds(target):
			holders = append(holders, other)
		case vm.spec != nil && vm.spec.TypedSpec().CPU.Slice == target.Name:
			selectors = append(selectors, other)
		}
	}

	holds := obs.vms[name].holds(target)

	switch {
	case len(holders) > 0:
		return fmt.Sprintf("exclusive %s is still held by %v", target, holders)
	case obs.unattributed(target):
		return fmt.Sprintf("exclusive %s still runs tasks no machine accounts for", target)
	case len(selectors) > 0 && !holds:
		return fmt.Sprintf("exclusive %s is also selected by %v", target, selectors)
	default:
		return ""
	}
}

// specPins is the union of the host CPU pins the specification would start a domain with.
func specPins(cpu hypervisor.VirtualMachineCPUSpec) (cpuset.CPUSet, error) {
	pins := cpuset.New()

	for _, pin := range cpu.Pins {
		set, err := hypervisorhelpers.ParseHostIDList(pin.CPUs, hypervisorhelpers.MaxHostCPUID)
		if err != nil {
			return pins, fmt.Errorf("vCPU %d: %w", pin.VCPU, err)
		}

		pins = pins.Union(set)
	}

	if cpu.EmulatorPin != "" {
		set, err := hypervisorhelpers.ParseHostIDList(cpu.EmulatorPin, hypervisorhelpers.MaxHostCPUID)
		if err != nil {
			return pins, fmt.Errorf("emulator: %w", err)
		}

		pins = pins.Union(set)
	}

	return pins, nil
}

// changes reports whether the admission alters any placement.
func (adm admission) changes() bool {
	return len(adm.grants) > 0 || len(adm.withdraw) > 0
}

// admit applies the admission; it must run with admission closed.
func (ctrl *CPUPartitionController) admit(ctx context.Context, r controller.Runtime, obs *observation) error {
	adm := planAdmission(obs)

	obs.applied.AdmissionErrors = adm.errors

	var errs error

	for _, name := range adm.withdraw {
		if err := withdrawPlacement(ctx, r, obs.placements[name]); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	for _, name := range slices.Sorted(maps.Keys(adm.grants)) {
		if err := safe.WriterModify(ctx, r, hypervisor.NewVirtualMachineCPUPlacement(hypervisor.NamespaceName, name),
			func(res *hypervisor.VirtualMachineCPUPlacement) error {
				*res.TypedSpec() = adm.grants[name]

				return nil
			}); err != nil {
			errs = errors.Join(errs, fmt.Errorf("error granting CPU placement %q: %w", name, err))
		}
	}

	return errs
}

// holds reports whether the machine's placement names the target.
func (vm *vmObservation) holds(target cpupartition.Target) bool {
	return vm.placement != nil && placementKey(vm.placement.TypedSpec()) == target.Key()
}
