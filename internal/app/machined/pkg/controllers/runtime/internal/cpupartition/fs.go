// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/cgroups/v3/cgroup2"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// CgroupFS is the coordinator's view of the cgroup v2 hierarchy and of the sysfs CPU inventory.
//
// Paths are relative to the cgroup root (e.g. "kubepods", "virtualmachines.partition/a.partition").
type CgroupFS interface {
	// Online returns the authoritative online CPU set.
	Online() (cpuset.CPUSet, error)
	// Exists reports whether the cgroup directory exists.
	Exists(path string) (bool, error)
	// Ensure creates the cgroup with the cpuset controller enabled, if missing.
	Ensure(path string) error
	// Remove deletes an empty cgroup.
	Remove(path string) error
	// CPUs reads cpuset.cpus (configured); empty when the cgroup inherits.
	CPUs(path string) (string, error)
	// Effective reads cpuset.cpus.effective.
	Effective(path string) (cpuset.CPUSet, error)
	// SetCPUs writes cpuset.cpus; an empty value restores inheritance.
	SetCPUs(path, cpus string) error
	// Populated reports the populated flag from cgroup.events.
	Populated(path string) (bool, error)
	// LeafEffective returns, for every leaf cgroup under path with a non-empty configured
	// cpuset.cpus, its effective set: the application-container barrier.
	LeafEffective(path string) (map[string]cpuset.CPUSet, error)
}

// CgroupPath returns the cgroup path of a target.
func (t Target) CgroupPath() string {
	switch t.Kind {
	case KindRoot:
		switch config.CPUPartitionRoot(t.Name) {
		case config.CPUPartitionRootInit:
			return constants.CgroupInit
		case config.CPUPartitionRootSystem:
			return constants.CgroupSystem
		case config.CPUPartitionRootPodRuntime:
			return constants.CgroupPodRuntimeRoot
		case config.CPUPartitionRootKubepods:
			return constants.CgroupKubepods
		case config.CPUPartitionRootTalosContainers:
			return constants.CgroupTalosContainersRoot
		case config.CPUPartitionRootVirtualMachines:
			return constants.CgroupVirtualMachines
		}
	case KindShared:
		return constants.CgroupVirtualMachines + "/" + SharedPartition
	case KindSlice:
		return constants.CgroupVirtualMachines + "/" + t.Name + ".partition"
	}

	return ""
}

// HostFS is the CgroupFS of the running host.
type HostFS struct {
	// CgroupRoot is the cgroup v2 mount point, e.g. /sys/fs/cgroup.
	CgroupRoot string
	// SysfsPath is the sysfs mount point, e.g. /sys.
	SysfsPath string
}

// Online implements CgroupFS.
func (h HostFS) Online() (cpuset.CPUSet, error) {
	data, err := os.ReadFile(filepath.Join(h.SysfsPath, "devices", "system", "cpu", "online"))
	if err != nil {
		return cpuset.New(), err
	}

	return cpuset.Parse(strings.TrimSpace(string(data)))
}

// Exists implements CgroupFS.
func (h HostFS) Exists(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(h.CgroupRoot, path))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return err == nil, err
}

// Ensure implements CgroupFS: cgroup2.NewManager only delegates cpuset when the resources name a
// cpuset.cpus value, and the mask is written later by SetCPUs, so the controller is toggled on
// explicitly up the ancestor chain; a slice created under a root no process has used is otherwise
// missing cpuset.cpus (observed live). The write is idempotent for ancestors already delegating.
func (h HostFS) Ensure(path string) error {
	manager, err := cgroup2.NewManager(h.CgroupRoot, "/"+path, &cgroup2.Resources{CPU: &cgroup2.CPU{}})
	if err != nil {
		return err
	}

	return manager.ToggleControllers([]string{"cpu", "cpuset"}, cgroup2.Enable)
}

// Remove implements CgroupFS.
func (h HostFS) Remove(path string) error {
	return os.Remove(filepath.Join(h.CgroupRoot, path))
}

// CPUs implements CgroupFS.
func (h HostFS) CPUs(path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(h.CgroupRoot, path, "cpuset.cpus"))

	return strings.TrimSpace(string(data)), err
}

// Effective implements CgroupFS.
func (h HostFS) Effective(path string) (cpuset.CPUSet, error) {
	data, err := os.ReadFile(filepath.Join(h.CgroupRoot, path, "cpuset.cpus.effective"))
	if err != nil {
		return cpuset.New(), err
	}

	return cpuset.Parse(strings.TrimSpace(string(data)))
}

// SetCPUs implements CgroupFS.
func (h HostFS) SetCPUs(path, cpus string) error {
	return os.WriteFile(filepath.Join(h.CgroupRoot, path, "cpuset.cpus"), []byte(cpus+"\n"), 0o644)
}

// Populated implements CgroupFS.
func (h HostFS) Populated(path string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(h.CgroupRoot, path, "cgroup.events"))
	if err != nil {
		return false, err
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		if key, value, ok := strings.Cut(line, " "); ok && key == "populated" {
			return value == "1", nil
		}
	}

	return false, fmt.Errorf("cgroup.events of %q has no populated entry", path)
}

// LeafEffective implements CgroupFS.
func (h HostFS) LeafEffective(path string) (map[string]cpuset.CPUSet, error) {
	root := filepath.Join(h.CgroupRoot, path)
	leaves := map[string]cpuset.CPUSet{}

	err := filepath.WalkDir(root, func(dir string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}

			return walkErr
		}

		if !entry.IsDir() || dir == root {
			return nil
		}

		set, isLeaf, readErr := leafEffective(dir)
		if readErr != nil || !isLeaf {
			return readErr
		}

		rel, relErr := filepath.Rel(root, dir)
		if relErr != nil {
			return relErr
		}

		leaves[rel] = set

		return nil
	})

	return leaves, err
}

// leafEffective returns the effective set of a leaf cgroup with an explicit cpuset.cpus; isLeaf is
// false for cgroups with children or without a configured mask.
func leafEffective(dir string) (cpuset.CPUSet, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return cpuset.New(), false, err
	}

	for _, child := range entries {
		if child.IsDir() {
			return cpuset.New(), false, nil
		}
	}

	configured, err := os.ReadFile(filepath.Join(dir, "cpuset.cpus"))
	if err != nil || strings.TrimSpace(string(configured)) == "" {
		return cpuset.New(), false, err
	}

	data, err := os.ReadFile(filepath.Join(dir, "cpuset.cpus.effective"))
	if err != nil {
		return cpuset.New(), false, err
	}

	set, err := cpuset.Parse(strings.TrimSpace(string(data)))

	return set, err == nil, err
}
