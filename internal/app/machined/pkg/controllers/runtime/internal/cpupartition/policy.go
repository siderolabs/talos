// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition

import (
	"fmt"

	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
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

	reserved *cpuset.CPUSet
}

// NewPolicy resolves the policy of the boot against the online CPUs.
func NewPolicy(spec *runtime.CPUPartitionSpecSpec, online cpuset.CPUSet) (Policy, error) {
	var policy Policy

	roots, err := parseRoots(spec, online)
	if err != nil {
		return policy, err
	}

	for _, root := range config.CPUPartitionRoots() {
		if set, ok := roots[Root(root)]; ok {
			policy.Masks = append(policy.Masks, Mask{Target: Root(root), CPUs: set})
		}
	}

	if kubepods, ok := roots[Root(constants.CgroupKubepods)]; ok {
		// The static CPU manager policy refuses to start without a reserved CPU.
		reserved := online.Difference(kubepods)
		if reserved.IsEmpty() {
			return policy, fmt.Errorf("%s: CPUs %q leave no online CPU outside kubepods", Root(constants.CgroupKubepods), kubepods)
		}

		policy.reserved = &reserved
	}

	vmRoot, ok := roots[VirtualMachinesRoot]
	if !ok {
		if len(spec.Slices) > 0 {
			return policy, fmt.Errorf("slices require the %q root", constants.CgroupVirtualMachinesRoot)
		}

		return policy, nil
	}

	return policy, policy.addSlices(spec, vmRoot, online)
}

func parseRoots(spec *runtime.CPUPartitionSpecSpec, online cpuset.CPUSet) (map[Target]cpuset.CPUSet, error) {
	roots := map[Target]cpuset.CPUSet{}

	for root, list := range spec.Roots {
		target, ok := ParseKey(root)
		if !ok || target.Kind != KindRoot {
			return nil, fmt.Errorf("unknown root %q", root)
		}

		set, err := parseCPUs(target, list, online)
		if err != nil {
			return nil, err
		}

		roots[target] = set
	}

	return roots, nil
}

// addSlices adds the shared remainder and the slices of the virtual machine root.
func (policy *Policy) addSlices(spec *runtime.CPUPartitionSpecSpec, vmRoot, online cpuset.CPUSet) error {
	shared := vmRoot
	exclusiveCPUs := cpuset.New()

	var plain, exclusive []Mask

	for _, slice := range spec.Slices {
		target := Slice(slice.Name)
		if target.CgroupPath() == "" {
			return fmt.Errorf("invalid slice name %q", slice.Name)
		}

		set, err := parseCPUs(target, slice.CPUs, online)
		if err != nil {
			return err
		}

		if !set.IsSubsetOf(vmRoot) {
			return fmt.Errorf("%s: CPUs %q leave the virtual machine root %q", target, set, vmRoot)
		}

		shared = shared.Difference(set)

		if slice.Exclusive {
			exclusive = append(exclusive, Mask{Target: target, CPUs: set, Exclusive: true, ExclusiveCPUs: set})
			exclusiveCPUs = exclusiveCPUs.Union(set)
		} else {
			plain = append(plain, Mask{Target: target, CPUs: set})
		}
	}

	policy.Partitioned = !exclusiveCPUs.IsEmpty()

	// The virtual machine root delegates the CPUs of its exclusive slices.
	for i := range policy.Masks {
		if policy.Masks[i].Target == VirtualMachinesRoot {
			policy.Masks[i].ExclusiveCPUs = exclusiveCPUs
		}
	}

	// A fully carved root has no shared remainder: machines without a slice have no CPUs to run on.
	if !shared.IsEmpty() {
		policy.Masks = append(policy.Masks, Mask{Target: Shared, CPUs: shared})
	}

	// Exclusive slices go last: the virtual machine root delegates their CPUs before they claim them.
	policy.Masks = append(append(policy.Masks, plain...), exclusive...)

	return nil
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
	status := runtime.CPUPartitionStatusSpec{Ready: true}

	if policy.reserved != nil {
		status.KubeletReservedCPUs = policy.reserved.String()
	}

	for _, mask := range policy.Masks {
		if mask.Target.Kind == KindRoot {
			continue
		}

		name := mask.Target.Name
		if mask.Target == Shared {
			name = runtimecfg.CPUPartitionSharedSliceName
		}

		status.VirtualMachinePools = append(status.VirtualMachinePools, runtime.CPUPartitionPoolSpec{
			Name:      name,
			CPUs:      mask.CPUs.String(),
			Partition: mask.Target.Partition(),
		})
	}

	return status
}

func parseCPUs(target Target, list string, online cpuset.CPUSet) (cpuset.CPUSet, error) {
	set, err := cpuset.Parse(list)
	if err != nil {
		return set, fmt.Errorf("%s: invalid CPU list %q: %w", target, list, err)
	}

	if set.IsEmpty() {
		return set, fmt.Errorf("%s: no CPUs", target)
	}

	if offline := set.Difference(online); !offline.IsEmpty() {
		return set, fmt.Errorf("%s: CPUs %q are offline", target, offline)
	}

	return set, nil
}
