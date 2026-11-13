// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition

import (
	"fmt"

	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VirtualMachinesRoot is the target of the virtual machine root.
var VirtualMachinesRoot = Root(constants.CgroupVirtualMachinesRoot)

// Mask is the CPUs one target of a policy is bounded to.
type Mask struct {
	Target Target
	CPUs   cpuset.CPUSet
	// Exclusive is set on an exclusive slice, which becomes a partition root.
	Exclusive bool
	// ExclusiveCPUs is the cpuset.cpus.exclusive of the target, empty when it has none.
	ExclusiveCPUs cpuset.CPUSet
}

// Policy is the CPU partition policy of the boot resolved against the online CPUs.
type Policy struct {
	// Masks are ordered parents first: the fixed roots, then the shared remainder and the slices of
	// the virtual machine root, exclusive slices last.
	Masks []Mask
	// Partitioned is set when an exclusive slice makes partition modes matter.
	Partitioned bool

	kubeletReservedCPUs string
}

// NewPolicy resolves the policy of the boot against the online CPUs.
//
// The spec is projected from a validated document, so only what depends on the CPUs online at boot
// is checked here.
func NewPolicy(spec *runtime.CPUPartitionSpecSpec, online cpuset.CPUSet) (Policy, error) {
	var policy Policy

	slices, err := sliceMasks(spec.Slices, online)
	if err != nil {
		return policy, err
	}

	sliced, exclusiveCPUs, vmRoot := cpuset.New(), cpuset.New(), cpuset.New()

	for _, mask := range slices {
		sliced = sliced.Union(mask.CPUs)
		exclusiveCPUs = exclusiveCPUs.Union(mask.ExclusiveCPUs)
	}

	for _, root := range config.CPUPartitionRoots() {
		list, ok := spec.Roots[string(root)]
		if !ok {
			continue
		}

		target := Root(root)

		set, err := parseCPUs(target, list, online)
		if err != nil {
			return policy, err
		}

		mask := Mask{Target: target, CPUs: set}

		switch target {
		case Root(constants.CgroupKubepods):
			// The static CPU manager policy refuses to start without a reserved CPU.
			reserved := online.Difference(set)
			if reserved.IsEmpty() {
				return policy, fmt.Errorf("%s: CPUs %q leave no online CPU outside kubepods", target, set)
			}

			policy.kubeletReservedCPUs = reserved.String()
		case VirtualMachinesRoot:
			// The virtual machine root delegates the CPUs of its exclusive slices.
			mask.ExclusiveCPUs = exclusiveCPUs
			vmRoot = set
		}

		policy.Masks = append(policy.Masks, mask)
	}

	// A fully carved root has no shared remainder: machines without a slice have no CPUs to run on.
	if shared := vmRoot.Difference(sliced); !shared.IsEmpty() {
		policy.Masks = append(policy.Masks, Mask{Target: Shared, CPUs: shared})
	}

	policy.Masks = append(policy.Masks, slices...)
	policy.Partitioned = !exclusiveCPUs.IsEmpty()

	return policy, nil
}

// sliceMasks resolves the slices of the virtual machine root. Exclusive slices go last: the virtual
// machine root delegates their CPUs before they claim them.
func sliceMasks(slices []runtime.CPUPartitionSliceSpec, online cpuset.CPUSet) ([]Mask, error) {
	var plain, exclusive []Mask

	for _, slice := range slices {
		target := Slice(slice.Name)

		set, err := parseCPUs(target, slice.CPUs, online)
		if err != nil {
			return nil, err
		}

		if slice.Exclusive {
			exclusive = append(exclusive, Mask{Target: target, CPUs: set, Exclusive: true, ExclusiveCPUs: set})
		} else {
			plain = append(plain, Mask{Target: target, CPUs: set})
		}
	}

	return append(plain, exclusive...), nil
}

// Effective returns the CPUs the target is expected to run on: the virtual machine root keeps only
// the CPUs its exclusive slices do not take.
func (mask Mask) Effective() cpuset.CPUSet {
	if mask.Target == VirtualMachinesRoot {
		return mask.CPUs.Difference(mask.ExclusiveCPUs)
	}

	return mask.CPUs
}

// Status is the ready status of the applied policy, with the CPU sets its consumers use.
func (policy Policy) Status() runtime.CPUPartitionStatusSpec {
	status := runtime.CPUPartitionStatusSpec{Ready: true, KubeletReservedCPUs: policy.kubeletReservedCPUs}

	for _, mask := range policy.Masks {
		if mask.Target.Kind == KindRoot {
			continue
		}

		status.VirtualMachinePools = append(status.VirtualMachinePools, runtime.CPUPartitionPoolSpec{
			Name:      mask.Target.Name,
			CPUs:      mask.CPUs.String(),
			Partition: "/" + mask.Target.CgroupPath(),
		})
	}

	return status
}

// parseCPUs refuses a target without a cgroup, so no mask is ever written outside its root or onto
// the shared slice.
func parseCPUs(target Target, list string, online cpuset.CPUSet) (cpuset.CPUSet, error) {
	if target.CgroupPath() == "" {
		return cpuset.New(), fmt.Errorf("%s: no cgroup", target)
	}

	set, err := cpuset.Parse(list)
	if err != nil {
		return set, fmt.Errorf("%s: invalid CPU list %q: %w", target, list, err)
	}

	if offline := set.Difference(online); !offline.IsEmpty() {
		return set, fmt.Errorf("%s: CPUs %q are offline", target, offline)
	}

	return set, nil
}
