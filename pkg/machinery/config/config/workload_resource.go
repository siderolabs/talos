// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// WorkloadResourceConfig defines the interface to access aggregate workload resource limits.
//
// Every limit is a percentage of the host memory minus the reserved memory; zero means the root is omitted.
type WorkloadResourceConfig interface {
	WorkloadResourceConfigSignal()

	MemoryReserved() optional.Optional[uint64]

	KubepodsMemoryPercent() uint64
	TalosContainersMemoryPercent() uint64
	VirtualMachinesMemoryPercent() uint64
}

// DefaultMemoryReserved is the reserved memory when WorkloadResourceConfig sets none: the kubelet's default
// systemReserved.memory, so every workload root defaults to the ceiling kubepods always had.
func DefaultMemoryReserved(machineType machine.Type) uint64 {
	if machineType.IsControlPlane() {
		return meta.MustByteSize(constants.KubeletSystemReservedMemoryControlPlane).Value()
	}

	return meta.MustByteSize(constants.KubeletSystemReservedMemoryWorker).Value()
}

// MemoryLimit resolves a memory percentage on a host with the given capacity into bytes; zero means not limited.
//
// A percentage which rounds down to zero resolves to one byte, so the page size check rejects it instead of
// lifting the limit.
func MemoryLimit(percent, capacity, reserved uint64) uint64 {
	if percent == 0 {
		return 0
	}

	available := capacity - min(reserved, capacity)

	return max(available/100*percent+available%100*percent/100, 1)
}
