// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cgroup provides cgroup utilities to handle nested cgroups.
//
// When Talos runs in a container, it might either share or not share the host cgroup namespace.
// If the cgroup namespace is not shared, PID 1 will appear in cgroup '/', otherwise it will be
// part of some pre-existing cgroup hierarchy.
//
// When Talos is running in a non-container mode, it is always at the root of the cgroup hierarchy.
//
// This package provides a transparent way to handle nested cgroups by providing a Path() function
// which returns the correct cgroup path based on the cgroup hierarchy available.
package cgroup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/go-debug"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/containermode"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// CommonCgroup interface presents a cgroup manager, be it v1 or v2
// It can be further extended once new methods are required.
type CommonCgroup interface {
	Delete() error
}

var root = "/"

// InitRoot initializes the root cgroup path.
//
// This function should be called once at the beginning of the program, after the cgroup
// filesystem is mounted.
//
// This function only supports cgroupv2 nesting.
func InitRoot() error {
	if cgroups.Mode() != cgroups.Unified {
		return nil
	}

	var err error

	root, err = cgroup2.NestedGroupPath("/")

	return err
}

// Root returns the root cgroup path.
func Root() string {
	return root
}

// Path returns the path to the
//
// This function handles the case when the cgroups are nested.
func Path(cgroupPath string) string {
	if cgroups.Mode() != cgroups.Unified {
		return cgroupPath
	}

	return filepath.Join(root, cgroupPath)
}

func zeroIfRace[T any](v T) T {
	if debug.RaceEnabled {
		var zeroT T

		return zeroT
	}

	return v
}

//nolint:gocyclo
func getCgroupV2Resources(name string) *cgroup2.Resources {
	switch name {
	case constants.CgroupInit:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupInitReservedMemory)),
				Low:  new(int64(constants.CgroupInitReservedMemory * 2)),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupInitMillicores))),
			},
		}
	case constants.CgroupSystem:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min: new(int64(constants.CgroupSystemReservedMemory)),
				Low: new(int64(constants.CgroupSystemReservedMemory * 2)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupSystemMillicores))),
			},
		}
	case constants.CgroupSystemDebug:
		return &cgroup2.Resources{} // no limits for debug cgroup
	case constants.CgroupSystemRuntime:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupSystemRuntimeReservedMemory)),
				Low:  new(int64(constants.CgroupSystemRuntimeReservedMemory * 2)),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupSystemRuntimeMillicores))),
			},
		}
	case constants.CgroupUdevd:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupUdevdReservedMemory)),
				Low:  new(int64(constants.CgroupUdevdReservedMemory * 2)),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupUdevdMillicores))),
			},
		}
	case constants.CgroupPodRuntimeRoot:
		return &cgroup2.Resources{
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupPodRuntimeRootMillicores))),
			},
		}
	case constants.CgroupPodRuntime:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupPodRuntimeReservedMemory)),
				Low:  new(int64(constants.CgroupPodRuntimeReservedMemory * 2)),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupPodRuntimeMillicores))),
			},
		}
	case constants.CgroupPodRuntimeShim:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupPodRuntimeShimReservedMemory)),
				Low:  new(int64(constants.CgroupPodRuntimeShimReservedMemory * 2)),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupPodRuntimeShimMillicores))),
			},
		}
	case constants.CgroupKubelet:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupKubeletReservedMemory)),
				Low:  new(int64(constants.CgroupKubeletReservedMemory * 2)),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupKubeletMillicores))),
			},
		}
	case constants.CgroupEtcd:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Low:  new(int64(constants.CgroupEtcdReservedMemory)),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupEtcdMillicores))),
			},
		}
	case constants.CgroupDashboard:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Max: zeroIfRace(new(int64(constants.CgroupDashboardMaxMemory))),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupDashboardMillicores))),
			},
		}
	case constants.CgroupApid:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupApidReservedMemory)),
				Low:  new(int64(constants.CgroupApidReservedMemory * 2)),
				Max:  zeroIfRace(new(int64(constants.CgroupApidMaxMemory))),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupApidMillicores))),
			},
		}
	case constants.CgroupTrustd:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupTrustdReservedMemory)),
				Low:  new(int64(constants.CgroupTrustdReservedMemory * 2)),
				Max:  zeroIfRace(new(int64(constants.CgroupTrustdMaxMemory))),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupTrustdMillicores))),
			},
		}
	case constants.CgroupSystemSandbox:
		return &cgroup2.Resources{
			Memory: &cgroup2.Memory{
				Min:  new(int64(constants.CgroupSystemSandboxReservedMemory)),
				Low:  new(int64(constants.CgroupSystemSandboxReservedMemory * 2)),
				Swap: new(int64(0)),
			},
			CPU: &cgroup2.CPU{
				Weight: new(MillicoresToCPUWeight(MilliCores(constants.CgroupSystemSandboxMillicores))),
			},
		}
	}

	return &cgroup2.Resources{}
}

// CreateCgroup creates a cgroup, with resources limits if configured and supported.
func CreateCgroup(name string) (CommonCgroup, error) {
	resources := getCgroupV2Resources(name)

	if containermode.InContainer() {
		// don't attempt to set resources in container mode, as they might conflict with the parent cgroup tree
		resources = &cgroup2.Resources{}
	}

	if cgroups.Mode() == cgroups.Unified {
		cg, err := cgroup2.NewManager(constants.CgroupMountPath, Path(name), resources)
		if err != nil {
			return nil, fmt.Errorf("failed to create cgroup: %w", err)
		}

		if name == constants.CgroupInit {
			if err := cg.AddProc(uint64(os.Getpid())); err != nil {
				return nil, fmt.Errorf("failed to move init process to cgroup: %w", err)
			}
		}

		return cg, nil
	}

	cg, err := cgroup1.New(cgroup1.StaticPath(name), &specs.LinuxResources{})
	if err != nil {
		return nil, fmt.Errorf("failed to create cgroup: %w", err)
	}

	if name == constants.CgroupInit {
		if err := cg.Add(cgroup1.Process{
			Pid: os.Getpid(),
		}); err != nil {
			return nil, fmt.Errorf("failed to move init process to cgroup: %w", err)
		}
	}

	return cg, nil
}

// KillCgroup kills all processes in the cgroup.
//
// This method is only supported for cgroupv2, as cgroupv1 is legacy and only in container mode.
func KillCgroup(log *zap.Logger, name string) error {
	if cgroups.Mode() != cgroups.Unified {
		return nil
	}

	cg, err := cgroup2.Load(Path(name), cgroup2.WithMountpoint(constants.CgroupMountPath))
	if err != nil {
		return fmt.Errorf("failed to load cgroup: %w", err)
	}

	for i := range 10 {
		pids, err := cg.Procs(true)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}

			return fmt.Errorf("failed to get processes in cgroup: %w", err)
		}

		if len(pids) == 0 {
			break
		}

		if i == 0 {
			log.Info(
				"killing processes in cgroup",
				zap.String("cgroup", name),
				zap.Int("num_processes", len(pids)),
			)

			if err := cg.Kill(); err != nil {
				return fmt.Errorf("failed to kill processes in cgroup: %w", err)
			}
		} else {
			log.Info(
				"waiting for processes in cgroup to exit",
				zap.String("cgroup", name),
				zap.Int("num_processes", len(pids)),
			)
		}

		<-time.After(time.Second)
	}

	return cg.Delete()
}
