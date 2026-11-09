// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package numa_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision/providers/qemu/internal/numa"
)

func TestMemoryArgs(t *testing.T) {
	for _, tt := range []struct {
		name         string
		nodes        int
		cpus, memory int64
		drivers      []string
		want         []string
	}{
		{name: "default", cpus: 4, memory: 1024},
		{name: "default shared once", cpus: 4, memory: 1024, drivers: []string{"virtiofs", "virtiofs"}, want: []string{
			"-object", "memory-backend-file,id=mem,size=1024M,mem-path=/shm,share=on", "-numa", "node,memdev=mem",
		}},
		{name: "one", nodes: 1, cpus: 1, memory: 1, want: []string{
			"-object", "memory-backend-ram,id=mem0,size=1M", "-numa", "node,nodeid=0,cpus=0-0,memdev=mem0",
		}},
		{name: "two", nodes: 2, cpus: 4, memory: 1024, want: []string{
			"-object", "memory-backend-ram,id=mem0,size=512M", "-numa", "node,nodeid=0,cpus=0-1,memdev=mem0",
			"-object", "memory-backend-ram,id=mem1,size=512M", "-numa", "node,nodeid=1,cpus=2-3,memdev=mem1",
		}},
		{name: "remainders", nodes: 3, cpus: 5, memory: 8, want: []string{
			"-object", "memory-backend-ram,id=mem0,size=3M", "-numa", "node,nodeid=0,cpus=0-1,memdev=mem0",
			"-object", "memory-backend-ram,id=mem1,size=3M", "-numa", "node,nodeid=1,cpus=2-3,memdev=mem1",
			"-object", "memory-backend-ram,id=mem2,size=2M", "-numa", "node,nodeid=2,cpus=4-4,memdev=mem2",
		}},
		{name: "shared remainders once", nodes: 3, cpus: 5, memory: 8, drivers: []string{"virtiofs", "virtiofs"}, want: []string{
			"-object", "memory-backend-file,id=mem0,size=3M,mem-path=/shm,offset=0,share=on", "-numa", "node,nodeid=0,cpus=0-1,memdev=mem0",
			"-object", "memory-backend-file,id=mem1,size=3M,mem-path=/shm,offset=3145728,share=on", "-numa", "node,nodeid=1,cpus=2-3,memdev=mem1",
			"-object", "memory-backend-file,id=mem2,size=2M,mem-path=/shm,offset=6291456,share=on", "-numa", "node,nodeid=2,cpus=4-4,memdev=mem2",
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args, err := numa.MemoryArgs(numa.Config{
				Nodes: tt.nodes, VCPUs: tt.cpus, MemoryMiB: tt.memory,
				DiskDrivers: tt.drivers, MemShmPath: "/shm",
			})
			require.NoError(t, err)
			require.Equal(t, tt.want, args)
		})
	}
}

func TestSMPArg(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		want  string
		cpus  int64
		nodes int
	}{
		{name: "unchanged default", cpus: 4, want: "cpus=4"},
		{name: "one node", cpus: 4, nodes: 1, want: "cpus=4,sockets=1,cores=4,threads=1"},
		{name: "one socket per node", cpus: 4, nodes: 2, want: "cpus=4,sockets=2,cores=2,threads=1"},
		{name: "uneven nodes", cpus: 5, nodes: 2, want: "cpus=5,sockets=5,cores=1,threads=1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.want, numa.SMPArg(test.cpus, test.nodes),
				"a CPU socket/cluster must not span NUMA nodes on ARM64")
		})
	}
}

func TestMemoryArgsInvalid(t *testing.T) {
	for _, config := range []numa.Config{
		{Nodes: -1, VCPUs: 4, MemoryMiB: 1024},
		{Nodes: 3, VCPUs: 2, MemoryMiB: 1024},
		{Nodes: 3, VCPUs: 4, MemoryMiB: 2},
		{Nodes: 1, VCPUs: 0, MemoryMiB: 1024},
		{Nodes: 1, VCPUs: 1, MemoryMiB: 0},
	} {
		_, err := numa.MemoryArgs(config)
		require.Error(t, err)
	}
}
