// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package startup

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/cgroups/v3"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// CreateSystemCgroups creates system cgroups.
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

	groups := []string{
		constants.CgroupInit,
		constants.CgroupSystem,
		constants.CgroupPodRuntimeRoot,
	}

	for _, c := range groups {
		_, err := cgroup.CreateCgroup(c)
		if err != nil {
			return err
		}
	}

	return next()(ctx, log, rt, next)
}
