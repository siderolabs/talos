// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"fmt"
	"math"
	"slices"
)

func nodeVCPUCount(nanoCPUs int64) int64 {
	return max(1, int64(math.RoundToEven(float64(nanoCPUs)/1000/1000/1000)))
}

func smpArg(config *LaunchConfig) string {
	if config.NUMANodes <= 0 {
		return fmt.Sprintf("cpus=%d", config.VCPUCount)
	}

	// ARM64 CPU clusters must not span NUMA nodes. Uneven allocations need
	// single-core sockets so every node boundary is also a socket boundary.
	cores := max(1, config.VCPUCount/int64(config.NUMANodes))
	if config.VCPUCount%int64(config.NUMANodes) != 0 {
		cores = 1
	}

	return fmt.Sprintf("cpus=%d,sockets=%d,cores=%d,threads=1", config.VCPUCount, config.VCPUCount/cores, cores)
}

func validateNUMANodes(count int, cpus, memoryMiB int64) error {
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

// memoryArgs allocates every vCPU and MiB, assigning remainders to the first nodes.
func memoryArgs(config *LaunchConfig) ([]string, error) {
	if err := validateNUMANodes(config.NUMANodes, config.VCPUCount, config.MemSize); err != nil {
		return nil, err
	}

	shared := slices.Contains(config.DiskDrivers, "virtiofs")

	if config.NUMANodes == 0 {
		if !shared {
			return nil, nil
		}

		return []string{
			"-object", fmt.Sprintf("memory-backend-file,id=mem,size=%dM,mem-path=%s,share=on", config.MemSize, config.MemShmPath),
			"-numa", "node,memdev=mem",
		}, nil
	}

	args := make([]string, 0, config.NUMANodes*4)
	count := int64(config.NUMANodes)

	var cpuStart, offset int64

	for node := range count {
		cpus := config.VCPUCount / count
		if node < config.VCPUCount%count {
			cpus++
		}

		memory := config.MemSize / count
		if node < config.MemSize%count {
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
