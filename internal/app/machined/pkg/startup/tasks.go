// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package startup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// LogMode prints the current mode.
func LogMode(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	log.Info("platform information", zap.Stringer("mode", rt.State().Platform().Mode()))
	log.Info("FIPS mode", zap.Bool("enabled", fipsmode.Enabled()), zap.Bool("strict", fipsmode.Strict()))

	return next()(ctx, log, rt, next)
}

// SetupSystemDirectories creates system default directories.
func SetupSystemDirectories(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	for _, dir := range []struct {
		path  string
		perm  os.FileMode
		label string
	}{
		{constants.SystemEtcPath, 0o700, constants.EtcSelinuxLabel},
		{constants.SystemVarPath, 0o700, constants.SystemVarSelinuxLabel},
		{constants.StateMountPoint, 0o700, ""},
		{constants.SystemRunPath, 0o751, "system_u:object_r:system_run_t:s0"},
		{"/system/run/containerd", 0o711, "system_u:object_r:sys_containerd_run_t:s0"},
		{"/run/containerd", 0o711, "system_u:object_r:pod_containerd_run_t:s0"},
	} {
		if err := os.MkdirAll(dir.path, dir.perm); err != nil {
			return fmt.Errorf("setupSystemDirectories: %w", err)
		}

		if dir.label != "" {
			if err := selinux.SetLabel(dir.path, dir.label); err != nil {
				return fmt.Errorf("setupSystemDirectories: %w", err)
			}
		}
	}

	return next()(ctx, log, rt, next)
}

// InitVolumeLifecycle initializes volume lifecycle resource.
func InitVolumeLifecycle(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	if err := rt.State().V1Alpha2().Resources().Create(ctx, block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID)); err != nil {
		return fmt.Errorf("initVolumeLifecycle: %w", err)
	}

	return next()(ctx, log, rt, next)
}

// MountCgroups represents mounts the cgroupfs (only in !container).
func MountCgroups(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	if rt.State().Platform().Mode().InContainer() {
		return next()(ctx, log, rt, next)
	}

	if pointer.SafeDeref(procfs.ProcCmdline().Get(constants.KernelParamCGroups).First()) == "0" {
		log.Warn(fmt.Sprintf("kernel argument %v is no longer supported", constants.KernelParamCGroups))
	}

	cgroup := mount.NewCgroup2()

	if _, err := cgroup.Mount(); err != nil {
		return fmt.Errorf("mountCgroups: %w", err)
	}

	defer func() {
		if err := cgroup.Unmount(); err != nil {
			log.Warn("failed to unmount cgroups", zap.Error(err))
		}
	}()

	return next()(ctx, log, rt, next)
}

// MountPseudoLate mounts the late pseudo filesystems (only in !container).
func MountPseudoLate(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	if rt.State().Platform().Mode().InContainer() {
		return next()(ctx, log, rt, next)
	}

	late := mount.PseudoLate(log.Sugar().Infof)

	unmounter, err := late.Mount()
	if err != nil {
		return fmt.Errorf("mountPseudoLate: %w", err)
	}

	defer func() {
		if err := unmounter(); err != nil {
			log.Warn("failed to unmount pseudo late", zap.Error(err))
		}
	}()

	return next()(ctx, log, rt, next)
}

// SetRLimit sets the file descriptor limit.
func SetRLimit(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	if rt.State().Platform().Mode().InContainer() {
		return next()(ctx, log, rt, next)
	}

	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: 1048576, Max: 1048576}); err != nil {
		return fmt.Errorf("setRLimit: %w", err)
	}

	return next()(ctx, log, rt, next)
}

// SetEnvironmentVariables sets the environment variables.
func SetEnvironmentVariables(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	// Set the PATH env var.
	if err := os.Setenv("PATH", constants.PATH); err != nil {
		return errors.New("error setting PATH")
	}

	if !rt.State().Platform().Mode().InContainer() {
		// in container mode, ignore cmdline
		for _, env := range environment.Get(nil) {
			key, val, _ := strings.Cut(env, "=")

			if err := os.Setenv(key, val); err != nil {
				return fmt.Errorf("error setting %s: %w", val, err)
			}
		}
	}

	return next()(ctx, log, rt, next)
}
