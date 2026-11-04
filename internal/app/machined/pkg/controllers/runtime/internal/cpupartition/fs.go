// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition

import (
	"errors"
	"fmt"
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
type CgroupFS interface { //nolint:interfacebloat // one method per cpuset interface file
	// Online returns the online CPU set.
	Online() (cpuset.CPUSet, error)
	// Ensure creates the cgroup if missing, with the cpu and cpuset controllers delegated to it.
	Ensure(path string) error
	// CPUs reads the configured cpuset.cpus; it is empty when the cgroup inherits its parent's.
	CPUs(path string) (cpuset.CPUSet, error)
	// Effective reads cpuset.cpus.effective.
	Effective(path string) (cpuset.CPUSet, error)
	// SetCPUs writes cpuset.cpus; an empty value restores inheritance.
	SetCPUs(path, cpus string) error
	// Populated reports the populated flag of cgroup.events.
	Populated(path string) (bool, error)
	// Partition reads cpuset.cpus.partition verbatim.
	Partition(path string) (string, error)
	// SetPartition writes cpuset.cpus.partition.
	SetPartition(path, partition string) error
	// Exclusive reads the configured cpuset.cpus.exclusive.
	Exclusive(path string) (cpuset.CPUSet, error)
	// SetExclusive writes cpuset.cpus.exclusive.
	SetExclusive(path, cpus string) error
	// ExclusiveEffective reads cpuset.cpus.exclusive.effective.
	ExclusiveEffective(path string) (cpuset.CPUSet, error)
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

// CPUs implements CgroupFS.
func (h HostFS) CPUs(path string) (cpuset.CPUSet, error) {
	return h.readCPUs(path, "cpuset.cpus")
}

// Effective implements CgroupFS.
func (h HostFS) Effective(path string) (cpuset.CPUSet, error) {
	return h.readCPUs(path, "cpuset.cpus.effective")
}

// SetCPUs implements CgroupFS.
//
// The file is never created: a missing cpuset.cpus means cpuset is not delegated to the cgroup.
func (h HostFS) SetCPUs(path, cpus string) error {
	return h.writeProperty(path, "cpuset.cpus", cpus)
}

// Partition reads cpuset.cpus.partition verbatim, so an invalid state keeps the kernel's reason,
// e.g. "root invalid (Parent is not a partition root)".
func (h HostFS) Partition(path string) (string, error) {
	return h.readProperty(path, "cpuset.cpus.partition")
}

// SetPartition writes cpuset.cpus.partition; the file is never created.
func (h HostFS) SetPartition(path, partition string) error {
	return h.writeProperty(path, "cpuset.cpus.partition", partition)
}

// Exclusive reads the configured cpuset.cpus.exclusive; it may be empty.
func (h HostFS) Exclusive(path string) (cpuset.CPUSet, error) {
	return h.readCPUs(path, "cpuset.cpus.exclusive")
}

// SetExclusive writes cpuset.cpus.exclusive; the file is never created.
func (h HostFS) SetExclusive(path, cpus string) error {
	return h.writeProperty(path, "cpuset.cpus.exclusive", cpus)
}

// ExclusiveEffective reads cpuset.cpus.exclusive.effective.
func (h HostFS) ExclusiveEffective(path string) (cpuset.CPUSet, error) {
	return h.readCPUs(path, "cpuset.cpus.exclusive.effective")
}

func (h HostFS) readCPUs(path, property string) (cpuset.CPUSet, error) {
	value, err := h.readProperty(path, property)
	if err != nil {
		return cpuset.New(), err
	}

	return cpuset.Parse(value)
}

func (h HostFS) readProperty(path, property string) (string, error) {
	data, err := os.ReadFile(filepath.Join(h.CgroupRoot, path, property))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// writeProperty never creates the file: a missing interface file means the controller is not
// delegated to the cgroup, or the kernel does not support the property.
func (h HostFS) writeProperty(path, property, value string) error {
	f, err := os.OpenFile(filepath.Join(h.CgroupRoot, path, property), os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}

	_, err = f.WriteString(value + "\n")
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
