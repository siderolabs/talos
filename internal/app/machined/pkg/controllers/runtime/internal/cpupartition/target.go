// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cpupartition maps CPU partition targets to cgroups and operates on the cgroup v2 hierarchy.
package cpupartition

import (
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// Kind of a partition target.
type Kind int

// Target kinds.
const (
	// KindRoot is one of the fixed roots listed by config.CPUPartitionRoots.
	KindRoot Kind = iota
	// KindSlice is a slice of the virtual machine root, the shared remainder included.
	KindSlice
)

// Target identifies one cgroup whose cpuset a CPU partition policy bounds.
type Target struct {
	Kind Kind
	Name string
}

// Root returns the target of a fixed root.
func Root(root config.CPUPartitionRoot) Target {
	return Target{Kind: KindRoot, Name: string(root)}
}

// Slice returns the target of a slice.
func Slice(name string) Target {
	return Target{Kind: KindSlice, Name: name}
}

// Shared is the target of the virtual machine root outside every named slice.
var Shared = Slice(runtimecfg.CPUPartitionSharedSliceName)

// String implements fmt.Stringer.
func (t Target) String() string {
	if t.Kind == KindSlice {
		return "slice " + t.Name
	}

	return "root " + t.Name
}

// CgroupPath returns the cgroup path of the target relative to the cgroup root.
//
// It is empty for an unknown root and for an invalid slice name, so a target never maps outside its root.
func (t Target) CgroupPath() string {
	switch {
	case t.Kind == KindSlice && hypervisorhelpers.ValidateName(t.Name) == nil:
		return constants.CgroupVirtualMachines + "/" + t.Name + ".partition"
	case t.Kind == KindSlice:
		return ""
	case t.Name == constants.CgroupVirtualMachinesRoot:
		return constants.CgroupVirtualMachines
	case slices.Contains(config.CPUPartitionRoots(), config.CPUPartitionRoot(t.Name)):
		return t.Name
	default:
		return ""
	}
}
