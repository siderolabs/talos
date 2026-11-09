// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package numa builds the QEMU arguments of an emulated NUMA topology.
package numa

import (
	"fmt"
	"slices"
)

// Config describes the guest resources split across the emulated NUMA nodes.
type Config struct {
	MemShmPath  string
	DiskDrivers []string
	VCPUs       int64
	MemoryMiB   int64
	Nodes       int
}

// SMPArg returns the value of the QEMU -smp argument.
func SMPArg(vcpus int64, nodes int) string {
	if nodes <= 0 {
		return fmt.Sprintf("cpus=%d", vcpus)
	}

	// ARM64 CPU clusters must not span NUMA nodes. Uneven allocations need
	// single-core sockets so every node boundary is also a socket boundary.
	cores := max(1, vcpus/int64(nodes))
	if vcpus%int64(nodes) != 0 {
		cores = 1
	}

	return fmt.Sprintf("cpus=%d,sockets=%d,cores=%d,threads=1", vcpus, vcpus/cores, cores)
}

// Validate checks that every NUMA node can receive at least one vCPU and one MiB.
func Validate(count int, cpus, memoryMiB int64) error {
	switch {
	case count < 0:
		return fmt.Errorf("NUMA node count must not be negative: %d", count)
	case count == 0:
		return nil
	case int64(count) > cpus:
		return fmt.Errorf("NUMA node count %d exceeds vCPU count %d", count, cpus)
	case int64(count) > memoryMiB:
		return fmt.Errorf("NUMA node count %d exceeds memory size %d MiB", count, memoryMiB)
	default:
		return nil
	}
}

// MemoryArgs allocates every vCPU and MiB, assigning remainders to the first nodes.
func MemoryArgs(config Config) ([]string, error) {
	if err := Validate(config.Nodes, config.VCPUs, config.MemoryMiB); err != nil {
		return nil, err
	}

	shared := slices.Contains(config.DiskDrivers, "virtiofs")

	if config.Nodes == 0 {
		if !shared {
			return nil, nil
		}

		return []string{
			"-object", fmt.Sprintf("memory-backend-file,id=mem,size=%dM,mem-path=%s,share=on", config.MemoryMiB, config.MemShmPath),
			"-numa", "node,memdev=mem",
		}, nil
	}

	args := make([]string, 0, config.Nodes*4)
	count := int64(config.Nodes)

	var cpuStart, offset int64

	for node := range count {
		cpus := config.VCPUs / count
		if node < config.VCPUs%count {
			cpus++
		}

		memory := config.MemoryMiB / count
		if node < config.MemoryMiB%count {
			memory++
		}

		backend := fmt.Sprintf("memory-backend-ram,id=mem%d,size=%dM", node, memory)
		if shared {
			// All nodes share disjoint regions of the existing, full-sized backing file.
			backend = fmt.Sprintf("memory-backend-file,id=mem%d,size=%dM,mem-path=%s,offset=%d,share=on", node, memory, config.MemShmPath, offset)
		}

		args = append(args, "-object", backend,
			"-numa", fmt.Sprintf("node,nodeid=%d,cpus=%d-%d,memdev=mem%d", node, cpuStart, cpuStart+cpus-1, node))
		cpuStart += cpus
		offset += memory * 1024 * 1024
	}

	return args, nil
}
