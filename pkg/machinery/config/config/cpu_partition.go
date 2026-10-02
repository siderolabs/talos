// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "github.com/siderolabs/talos/pkg/machinery/constants"

// CPUPartitionRoot names one of the fixed cgroup roots a CPU partition may bound.
type CPUPartitionRoot string

// CPUPartitionRoots lists every fixed root, in reporting order.
func CPUPartitionRoots() []CPUPartitionRoot {
	return []CPUPartitionRoot{
		constants.CgroupInit,
		constants.CgroupSystem,
		constants.CgroupPodRuntimeRoot,
		constants.CgroupKubepods,
		constants.CgroupTalosContainersRoot,
		constants.CgroupVirtualMachinesRoot,
	}
}

// CPUPartitionConfig defines the interface to access machine-wide CPU partitioning configuration.
//
// Every CPU list returned is canonical (sorted, deduplicated, ranges collapsed) once the document
// has passed validation.
type CPUPartitionConfig interface {
	CPUPartitionConfigSignal()

	// Omitted roots are unrestricted.
	Roots() map[CPUPartitionRoot]string
	Slices() []CPUPartitionSliceConfig
}

// CPUPartitionSliceConfig is one named subset of the virtual machine root.
type CPUPartitionSliceConfig interface {
	Name() string
	CPUs() string
	Exclusive() bool
}
