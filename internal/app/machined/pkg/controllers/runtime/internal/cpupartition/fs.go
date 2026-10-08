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

	"github.com/siderolabs/talos/internal/pkg/cgroups"
)

// CgroupFS is the view of the cgroup v2 hierarchy and of the sysfs CPU inventory CPU partitioning operates on.
//
// Paths are relative to the cgroup root, e.g. "kubepods" or "virtualmachines.partition/shared.partition".
type CgroupFS interface {
	// Online returns the online CPU set.
	Online() (cpuset.CPUSet, error)
	// Exists reports whether the cgroup exists.
	Exists(path string) (bool, error)
	// Ensure creates the cgroup if missing, with the cpu and cpuset controllers delegated to it.
	Ensure(path string) error
	// Remove deletes an empty cgroup.
	Remove(path string) error
	// CPUs reads the configured cpuset.cpus; it is empty when the cgroup inherits its parent's.
	CPUs(path string) (string, error)
	// Effective reads cpuset.cpus.effective.
	Effective(path string) (cpuset.CPUSet, error)
	// SetCPUs writes cpuset.cpus; an empty value restores inheritance.
	SetCPUs(path, cpus string) error
	// Populated reports the populated flag of cgroup.events.
	Populated(path string) (bool, error)
	// LeafEffective returns the effective set of every leaf cgroup under path with a configured
	// cpuset.cpus, keyed by its path relative to path.
	LeafEffective(path string) (map[string]cpuset.CPUSet, error)
	// Children returns the sorted names of the direct child cgroups; none when path does not exist.
	Children(path string) ([]string, error)
}

var _ CgroupFS = HostFS{}

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

// Ensure implements CgroupFS.
//
// Controllers are delegated top-down, and each parent delegates before its child is created or
// reused: a cgroup created under a parent which never enabled cpuset has no cpuset.cpus, and
// cgroup2.NewManager only enables cpuset when it writes a mask itself.
func (h HostFS) Ensure(path string) error {
	if path == "" {
		return errors.New("cgroup path is empty")
	}

	if err := cgroup2.VerifyGroupPath("/" + path); err != nil {
		return fmt.Errorf("invalid cgroup path %q: %w", path, err)
	}

	dir := h.CgroupRoot

	for component := range strings.SplitSeq(path, "/") {
		if err := delegate(dir); err != nil {
			return err
		}

		dir = filepath.Join(dir, component)

		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}

// delegate enables the cpu and cpuset controllers for the children of dir.
func delegate(dir string) error {
	f, err := os.OpenFile(filepath.Join(dir, "cgroup.subtree_control"), os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("error delegating cpu and cpuset in %q: %w", dir, err)
	}

	_, err = f.WriteString("+cpu +cpuset")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return fmt.Errorf("error delegating cpu and cpuset in %q: %w", dir, err)
	}

	return nil
}

// Remove implements CgroupFS.
func (h HostFS) Remove(path string) error {
	return os.Remove(filepath.Join(h.CgroupRoot, path))
}

// CPUs implements CgroupFS.
func (h HostFS) CPUs(path string) (string, error) {
	node, err := cgroups.GetCgroupProperty(filepath.Join(h.CgroupRoot, path), "cpuset.cpus")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(node.CPUSetCPUs)), nil
}

// Effective implements CgroupFS.
func (h HostFS) Effective(path string) (cpuset.CPUSet, error) {
	return effective(filepath.Join(h.CgroupRoot, path))
}

func effective(dir string) (cpuset.CPUSet, error) {
	node, err := cgroups.GetCgroupProperty(dir, "cpuset.cpus.effective")
	if err != nil {
		return cpuset.New(), err
	}

	return cpuset.Parse(strings.TrimSpace(string(node.CPUSetCPUsEffective)))
}

// SetCPUs implements CgroupFS.
//
// The file is never created: a missing cpuset.cpus means cpuset is not delegated to the cgroup.
func (h HostFS) SetCPUs(path, cpus string) error {
	f, err := os.OpenFile(filepath.Join(h.CgroupRoot, path, "cpuset.cpus"), os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}

	_, err = f.WriteString(cpus + "\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}

	return err
}

// Populated implements CgroupFS.
func (h HostFS) Populated(path string) (bool, error) {
	node, err := cgroups.GetCgroupProperty(filepath.Join(h.CgroupRoot, path), "cgroup.events")
	if err != nil {
		return false, err
	}

	populated, ok := node.CgroupEvents["populated"]
	if !ok {
		return false, fmt.Errorf("cgroup.events of %q has no populated entry", path)
	}

	return populated.Val == 1, nil
}

// Children implements CgroupFS.
func (h HostFS) Children(path string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(h.CgroupRoot, path))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	var children []string

	for _, entry := range entries {
		if entry.IsDir() {
			children = append(children, entry.Name())
		}
	}

	return children, nil
}

// LeafEffective implements CgroupFS.
//
// A leaf which cannot be read fails the walk, so callers never treat an unread leaf as released.
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

		set, isLeaf, err := leafEffective(dir)
		if err != nil || !isLeaf {
			return err
		}

		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return err
		}

		leaves[rel] = set

		return nil
	})

	return leaves, err
}

// leafEffective returns the effective set of dir if it has no child cgroups and a configured cpuset.cpus.
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

	node, err := cgroups.GetCgroupProperty(dir, "cpuset.cpus")
	if err != nil || strings.TrimSpace(string(node.CPUSetCPUs)) == "" {
		return cpuset.New(), false, err
	}

	set, err := effective(dir)

	return set, err == nil, err
}
