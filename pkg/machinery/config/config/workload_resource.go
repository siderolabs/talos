// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"strconv"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// WorkloadResourceConfig defines the interface to access aggregate workload resource limits.
type WorkloadResourceConfig interface {
	WorkloadResourceConfigSignal()

	MemoryReserved() optional.Optional[uint64]

	KubepodsMemoryLimit() MemoryLimit
	TalosContainersMemoryLimit() MemoryLimit
	VirtualMachinesMemoryLimit() MemoryLimit
}

// MemoryLimit is a workload root memory limit: an absolute size, or a percentage of the host memory minus
// the reserved memory. The zero value means the root is omitted.
type MemoryLimit struct {
	Bytes   uint64
	Percent uint64
}

// IsZero reports whether the root is omitted.
func (l MemoryLimit) IsZero() bool {
	return l.Bytes == 0 && l.Percent == 0
}

// String implements fmt.Stringer.
func (l MemoryLimit) String() string {
	if l.Percent != 0 {
		return strconv.FormatUint(l.Percent, 10) + "%"
	}

	return strconv.FormatUint(l.Bytes, 10) + " bytes"
}

// Resolve returns the limit in bytes on a host with the given capacity; zero means not limited.
//
// A percentage which rounds down to zero resolves to one byte, so the page size check rejects it instead of
// lifting the limit.
func (l MemoryLimit) Resolve(capacity, reserved uint64) uint64 {
	if l.Percent == 0 {
		return l.Bytes
	}

	available := capacity - min(reserved, capacity)

	return max(available/100*l.Percent+available%100*l.Percent/100, 1)
}

// ResolveMemoryReserved returns the reserved memory in bytes: WorkloadResourceConfig reserved.memory, or the default
// for the machine type.
func ResolveMemoryReserved(cfg Config) uint64 {
	var machineType machine.Type

	if cfg.Machine() != nil {
		machineType = cfg.Machine().Type()
	}

	reserved := DefaultMemoryReserved(machineType).Value()

	if resources := cfg.WorkloadResourceConfig(); resources != nil {
		reserved = resources.MemoryReserved().ValueOr(reserved)
	}

	return reserved
}

// DefaultMemoryReserved is the reserved memory when WorkloadResourceConfig sets none: the kubelet's default
// systemReserved.memory, so every workload root defaults to the ceiling kubepods always had.
func DefaultMemoryReserved(machineType machine.Type) meta.ByteSize {
	if machineType.IsControlPlane() {
		return meta.MustByteSize(constants.KubeletSystemReservedMemoryControlPlane)
	}

	return meta.MustByteSize(constants.KubeletSystemReservedMemoryWorker)
}
