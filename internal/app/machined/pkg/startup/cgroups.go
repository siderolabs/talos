// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package startup

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/go-debug"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func zeroIfRace[T any](v T) T {
	if debug.RaceEnabled {
		var zeroT T

		return zeroT
	}

	return v
}

// CreateSystemCgroups creates system cgroups.
//
//nolint:gocyclo
func CreateSystemCgroups(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	// in container mode cgroups mode depends on cgroups provided by the container runtime
	if !rt.State().Platform().Mode().InContainer() {
		// assert that cgroupsv2 is being used when running not in container mode,
		// as Talos sets up cgroupsv2 on its own
		if cgroups.Mode() != cgroups.Unified {
			return errors.New("cgroupsv2 should be used")
		}
	}

	// Initialize cgroups root path.
	if err := cgroup.InitRoot(); err != nil {
		return fmt.Errorf("error initializing cgroups root path: %w", err)
	}

	log.Info("initializing cgroups", zap.String("root", cgroup.Root()))

	groups := []struct {
		name      string
		resources *cgroup2.Resources
	}{
		{
			name: constants.CgroupInit,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Min: pointer.To[int64](constants.CgroupInitReservedMemory),
					Low: pointer.To[int64](constants.CgroupInitReservedMemory * 2),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupInitMillicores))),
				},
			},
		},
		{
			name: constants.CgroupSystem,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Min: pointer.To[int64](constants.CgroupSystemReservedMemory),
					Low: pointer.To[int64](constants.CgroupSystemReservedMemory * 2),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupSystemMillicores))),
				},
			},
		},
		{
			name: constants.CgroupSystemRuntime,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Min: pointer.To[int64](constants.CgroupSystemRuntimeReservedMemory),
					Low: pointer.To[int64](constants.CgroupSystemRuntimeReservedMemory * 2),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupSystemRuntimeMillicores))),
				},
			},
		},
		{
			name: constants.CgroupUdevd,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Min: pointer.To[int64](constants.CgroupUdevdReservedMemory),
					Low: pointer.To[int64](constants.CgroupUdevdReservedMemory * 2),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupUdevdMillicores))),
				},
			},
		},
		{
			name: constants.CgroupPodRuntimeRoot,
			resources: &cgroup2.Resources{
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupPodRuntimeRootMillicores))),
				},
			},
		},
		{
			name: constants.CgroupPodRuntime,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Min: pointer.To[int64](constants.CgroupPodRuntimeReservedMemory),
					Low: pointer.To[int64](constants.CgroupPodRuntimeReservedMemory * 2),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupPodRuntimeMillicores))),
				},
			},
		},
		{
			name: constants.CgroupKubelet,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Min: pointer.To[int64](constants.CgroupKubeletReservedMemory),
					Low: pointer.To[int64](constants.CgroupKubeletReservedMemory * 2),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupKubeletMillicores))),
				},
			},
		},
		{
			name: constants.CgroupDashboard,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Max: zeroIfRace(pointer.To[int64](constants.CgroupDashboardMaxMemory)),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupDashboardMillicores))),
				},
			},
		},
		{
			name: constants.CgroupApid,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Min: pointer.To[int64](constants.CgroupApidReservedMemory),
					Low: pointer.To[int64](constants.CgroupApidReservedMemory * 2),
					Max: zeroIfRace(pointer.To[int64](constants.CgroupApidMaxMemory)),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupApidMillicores))),
				},
			},
		},
		{
			name: constants.CgroupTrustd,
			resources: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Min: pointer.To[int64](constants.CgroupTrustdReservedMemory),
					Low: pointer.To[int64](constants.CgroupTrustdReservedMemory * 2),
					Max: zeroIfRace(pointer.To[int64](constants.CgroupTrustdMaxMemory)),
				},
				CPU: &cgroup2.CPU{
					Weight: pointer.To[uint64](cgroup.MillicoresToCPUWeight(cgroup.MilliCores(constants.CgroupTrustdMillicores))),
				},
			},
		},
	}

	for _, c := range groups {
		if cgroups.Mode() == cgroups.Unified {
			resources := c.resources

			if rt.State().Platform().Mode().InContainer() {
				// don't attempt to set resources in container mode, as they might conflict with the parent cgroup tree
				resources = &cgroup2.Resources{}
			}

			cg, err := cgroup2.NewManager(constants.CgroupMountPath, cgroup.Path(c.name), resources)
			if err != nil {
				return fmt.Errorf("failed to create cgroup: %w", err)
			}

			if c.name == constants.CgroupInit {
				if err := cg.AddProc(uint64(os.Getpid())); err != nil {
					return fmt.Errorf("failed to move init process to cgroup: %w", err)
				}
			}
		} else {
			cg, err := cgroup1.New(cgroup1.StaticPath(c.name), &specs.LinuxResources{})
			if err != nil {
				return fmt.Errorf("failed to create cgroup: %w", err)
			}

			if c.name == constants.CgroupInit {
				if err := cg.Add(cgroup1.Process{
					Pid: os.Getpid(),
				}); err != nil {
					return fmt.Errorf("failed to move init process to cgroup: %w", err)
				}
			}
		}
	}

	return next()(ctx, log, rt, next)
}
