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
	"github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// LogMode prints the current mode.
func LogMode(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	log.Info("platform information", zap.Stringer("mode", rt.State().Platform().Mode()))

	return next()(ctx, log, rt, next)
}

// SetupSystemDirectories creates system default directories.
func SetupSystemDirectories(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	for _, path := range []string{constants.SystemEtcPath, constants.SystemVarPath, constants.StateMountPoint} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("setupSystemDirectories: %w", err)
		}

		var label string

		switch path {
		case constants.SystemEtcPath:
			label = constants.EtcSelinuxLabel
		case constants.SystemVarPath:
			label = constants.SystemVarSelinuxLabel
		default: // /system/state is another mount
			label = ""
		}

		if err := selinux.SetLabel(path, label); err != nil {
			return err
		}
	}

	for _, path := range []string{constants.SystemRunPath} {
		if err := os.MkdirAll(path, 0o751); err != nil {
			return fmt.Errorf("setupSystemDirectories: %w", err)
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

	unmounter, err := mount.CGroupMountPoints().Mount()
	if err != nil {
		return fmt.Errorf("mountCgroups: %w", err)
	}

	defer func() {
		if err := unmounter(); err != nil {
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

	unmounter, err := mount.PseudoLate().Mount()
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
