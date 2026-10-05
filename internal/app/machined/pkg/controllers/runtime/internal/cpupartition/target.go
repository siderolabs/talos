// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cpupartition maps CPU partition targets to cgroups and operates on the cgroup v2 hierarchy.
package cpupartition

import (
	"fmt"
	"strings"

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
	// KindShared is the remainder of the virtual machine root outside every named slice.
	KindShared
	// KindSlice is a named slice of the virtual machine root.
	KindSlice
)

// SharedPartition is the cgroup name of the shared slice under the virtual machine root.
const SharedPartition = runtimecfg.CPUPartitionSharedSliceName + ".partition"

// Target identifies one cgroup whose cpuset a CPU partition policy bounds.
//
// Name is the root name for KindRoot, the slice name for KindSlice and empty for KindShared.
type Target struct {
	Kind Kind
	Name string
}

// Root returns the target of a fixed root.
func Root(root config.CPUPartitionRoot) Target {
	return Target{Kind: KindRoot, Name: string(root)}
}

// Slice returns the target of a named slice.
func Slice(name string) Target {
	return Target{Kind: KindSlice, Name: name}
}

// Shared is the target of the shared remainder.
var Shared = Target{Kind: KindShared}

// String implements fmt.Stringer.
func (t Target) String() string {
	switch t.Kind {
	case KindRoot:
		return "root " + t.Name
	case KindShared:
		return "shared slice"
	case KindSlice:
		return "slice " + t.Name
	default:
		return fmt.Sprintf("target(%d,%q)", t.Kind, t.Name)
	}
}

// Key is the stable string form of the target used in CPUPartitionStatus.
func (t Target) Key() string {
	switch t.Kind {
	case KindRoot:
		return t.Name
	case KindShared:
		return constants.CgroupVirtualMachinesRoot + "/" + runtimecfg.CPUPartitionSharedSliceName
	case KindSlice:
		return constants.CgroupVirtualMachinesRoot + "/" + t.Name
	default:
		return ""
	}
}

// ParseKey is the inverse of Key.
func ParseKey(key string) (Target, bool) {
	if child, ok := strings.CutPrefix(key, constants.CgroupVirtualMachinesRoot+"/"); ok {
		if child == runtimecfg.CPUPartitionSharedSliceName {
			return Shared, true
		}

		slice := Slice(child)

		return slice, slice.CgroupPath() != ""
	}

	for _, root := range config.CPUPartitionRoots() {
		if key == string(root) {
			return Root(root), true
		}
	}

	return Target{}, false
}

// CgroupPath returns the cgroup path of the target relative to the cgroup root.
//
// It is empty for an unknown root and for a slice name the configuration rejects, so a target
// never maps outside its root or onto the shared slice.
func (t Target) CgroupPath() string {
	switch t.Kind {
	case KindRoot:
		return rootCgroupPath(t.Name)
	case KindShared:
		return constants.CgroupVirtualMachines + "/" + SharedPartition
	case KindSlice:
		if hypervisorhelpers.ValidateName(t.Name) == nil && t.Name != runtimecfg.CPUPartitionSharedSliceName {
			return constants.CgroupVirtualMachines + "/" + t.Name + ".partition"
		}
	}

	return ""
}

func rootCgroupPath(root string) string {
	switch root {
	case constants.CgroupInit:
		return constants.CgroupInit
	case constants.CgroupSystem:
		return constants.CgroupSystem
	case constants.CgroupPodRuntimeRoot:
		return constants.CgroupPodRuntimeRoot
	case constants.CgroupKubepods:
		return constants.CgroupKubepods
	case constants.CgroupTalosContainersRoot:
		return constants.CgroupTalosContainersRoot
	case constants.CgroupVirtualMachinesRoot:
		return constants.CgroupVirtualMachines
	default:
		return ""
	}
}

// Partition returns the libvirt resource partition of a target virtual machines run in: the
// virtual machine root, the shared slice or a named slice. It is empty for every other target.
func (t Target) Partition() string {
	if t.Kind == KindRoot && t.Name != constants.CgroupVirtualMachinesRoot {
		return ""
	}

	if path := t.CgroupPath(); path != "" {
		return "/" + path
	}

	return ""
}
