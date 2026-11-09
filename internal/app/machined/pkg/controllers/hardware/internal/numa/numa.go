// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package numa reads the Linux NUMA topology from sysfs and procfs.
package numa

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/prometheus/procfs"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// ErrTopologyChanged marks a snapshot raced by hotplug, which is expected and can be retried.
var ErrTopologyChanged = errors.New("changed while reading NUMA topology")

// Read returns a snapshot of the NUMA topology.
func Read(sysfsPath, procfsPath string) (*hardware.NUMATopologySpec, error) { //nolint:gocyclo
	cpuPath := filepath.Join(sysfsPath, "devices/system/cpu")
	nodePath := filepath.Join(sysfsPath, "devices/system/node")

	present, err := readCPUSet(filepath.Join(cpuPath, "present"))
	if err != nil {
		return nil, err
	}

	online, err := readCPUSet(filepath.Join(cpuPath, "online"))
	if err != nil {
		return nil, err
	}

	for _, id := range online {
		if !slices.Contains(present, id) {
			return nil, fmt.Errorf("online CPU %d is not present", id)
		}
	}

	spec := &hardware.NUMATopologySpec{PresentCPUs: present, OnlineCPUs: online}
	_, err = os.Stat(nodePath)

	fallback := os.IsNotExist(err)
	if err != nil && !fallback {
		return nil, err
	}

	var nodes []uint32

	if fallback {
		fs, fsErr := procfs.NewFS(procfsPath)
		if fsErr != nil {
			return nil, fsErr
		}

		mem, memErr := fs.Meminfo()
		if memErr != nil {
			return nil, memErr
		}

		if mem.MemTotalBytes == nil {
			return nil, errors.New("MemTotal is missing from /proc/meminfo")
		}

		spec.Nodes = []hardware.NUMANodeSpec{{ID: 0, CPUs: slices.Clone(present), MemoryTotalBytes: *mem.MemTotalBytes}}
	} else {
		nodes, err = readCPUSet(filepath.Join(nodePath, "online"))
		if err != nil {
			return nil, err
		}

		if len(nodes) == 0 {
			return nil, errors.New("no online NUMA nodes")
		}

		spec.Nodes, err = readNodes(nodePath, nodes, present)
		if err != nil {
			return nil, err
		}
	}

	for _, cpu := range online {
		found := false
		for _, node := range spec.Nodes {
			found = found || slices.Contains(node.CPUs, cpu)
		}

		if !found {
			return nil, fmt.Errorf("online CPU %d has no NUMA node", cpu)
		}
	}

	if err := ValidateMembership(sysfsPath, spec, nodes, fallback); err != nil {
		return nil, err
	}

	return spec, nil
}

// ValidateMembership rechecks membership after collecting the nodes. Sysfs is not an atomic snapshot.
//
// fallback reports that the snapshot was taken without the sysfs node interface.
func ValidateMembership(sysfsPath string, spec *hardware.NUMATopologySpec, nodes []uint32, fallback bool) error { //nolint:gocyclo
	cpuPath := filepath.Join(sysfsPath, "devices/system/cpu")
	nodePath := filepath.Join(sysfsPath, "devices/system/node")

	afterPresent, err := readCPUSet(filepath.Join(cpuPath, "present"))
	if err != nil {
		return err
	}

	afterOnline, err := readCPUSet(filepath.Join(cpuPath, "online"))
	if err != nil {
		return err
	}

	if !slices.Equal(spec.PresentCPUs, afterPresent) || !slices.Equal(spec.OnlineCPUs, afterOnline) {
		return fmt.Errorf("CPU membership %w", ErrTopologyChanged)
	}

	if fallback {
		if _, err := os.Stat(nodePath); !os.IsNotExist(err) {
			return fmt.Errorf("NUMA node interface %w", ErrTopologyChanged)
		}

		return nil
	}

	afterNodes, err := readCPUSet(filepath.Join(nodePath, "online"))
	if err != nil {
		return err
	}

	if !slices.Equal(nodes, afterNodes) {
		return fmt.Errorf("node membership %w", ErrTopologyChanged)
	}

	for _, node := range spec.Nodes {
		cpus, err := readCPUSet(filepath.Join(nodePath, fmt.Sprintf("node%d/cpulist", node.ID)))
		if err != nil {
			return err
		}

		if !slices.Equal(node.CPUs, cpus) {
			return fmt.Errorf("node CPU membership %w", ErrTopologyChanged)
		}
	}

	return nil
}

func readCPUSet(path string) ([]uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	set, err := cpuset.Parse(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	result := make([]uint32, 0, set.Size())
	for _, id := range set.List() {
		if uint64(id) > math.MaxUint32 {
			return nil, fmt.Errorf("ID %d in %s exceeds uint32", id, path)
		}

		result = append(result, uint32(id))
	}

	return result, nil
}

func readNodes(nodePath string, ids, present []uint32) ([]hardware.NUMANodeSpec, error) {
	nodes := make([]hardware.NUMANodeSpec, 0, len(ids))
	assigned := make(map[uint32]struct{})

	for _, id := range ids {
		path := filepath.Join(nodePath, fmt.Sprintf("node%d", id))

		cpus, err := readCPUSet(filepath.Join(path, "cpulist"))
		if err != nil {
			return nil, err
		}

		for _, cpu := range cpus {
			if !slices.Contains(present, cpu) {
				return nil, fmt.Errorf("node %d CPU %d is not present", id, cpu)
			}

			if _, exists := assigned[cpu]; exists {
				return nil, fmt.Errorf("CPU %d belongs to multiple nodes", cpu)
			}

			assigned[cpu] = struct{}{}
		}

		memory, err := readNodeMemTotal(filepath.Join(path, "meminfo"))
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, hardware.NUMANodeSpec{ID: id, CPUs: cpus, MemoryTotalBytes: memory})
	}

	return nodes, nil
}

// readNodeMemTotal returns the total memory from the "Node N MemTotal: X kB" record of a node meminfo file.
func readNodeMemTotal(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != "MemTotal:" {
			continue
		}

		if len(fields) != 5 || fields[4] != "kB" {
			return 0, fmt.Errorf("malformed MemTotal record in %s: %q", path, strings.TrimSpace(line))
		}

		total, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse MemTotal in %s: %w", path, err)
		}

		if total > math.MaxUint64/1024 {
			return 0, fmt.Errorf("MemTotal in %s overflows: %d kB", path, total)
		}

		return total * 1024, nil
	}

	return 0, fmt.Errorf("MemTotal is missing from %s", path)
}
