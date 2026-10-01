// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// CPUPartitionRoot names one of the fixed cgroup roots a CPU partition may bound.
type CPUPartitionRoot string

// The fixed roots, in the order they are reported.
const (
	CPUPartitionRootInit            CPUPartitionRoot = "init"
	CPUPartitionRootSystem          CPUPartitionRoot = "system"
	CPUPartitionRootPodRuntime      CPUPartitionRoot = "podruntime"
	CPUPartitionRootKubepods        CPUPartitionRoot = "kubepods"
	CPUPartitionRootTalosContainers CPUPartitionRoot = "taloscontainers"
	CPUPartitionRootVirtualMachines CPUPartitionRoot = "virtualMachines"
)

// CPUPartitionRoots lists every fixed root, in reporting order.
func CPUPartitionRoots() []CPUPartitionRoot {
	return []CPUPartitionRoot{
		CPUPartitionRootInit,
		CPUPartitionRootSystem,
		CPUPartitionRootPodRuntime,
		CPUPartitionRootKubepods,
		CPUPartitionRootTalosContainers,
		CPUPartitionRootVirtualMachines,
	}
}

// CPUPartitionConfig defines the interface to access machine-wide CPU partitioning configuration.
//
// Every CPU list returned is canonical (sorted, deduplicated, ranges collapsed) once the document
// has passed validation.
type CPUPartitionConfig interface {
	// Marker for findMatchingDocs[T]
	CPUPartitionConfigSignal()

	// Roots returns the CPU list of every root that is explicitly bounded, keyed by root.
	//
	// A root absent from the map carries no restriction.
	Roots() map[CPUPartitionRoot]string
	// Slices returns the named virtual machine slices, in declaration order.
	Slices() []CPUPartitionSliceConfig
}

// CPUPartitionSliceConfig is one named subset of the virtual machine root.
type CPUPartitionSliceConfig interface {
	// Name of the slice, referenced by VirtualMachineConfig cpu.slice.
	Name() string
	// CPUs is the canonical host CPU list of the slice.
	CPUs() string
	// Exclusive slices must not overlap any other bounded root.
	Exclusive() bool
}
