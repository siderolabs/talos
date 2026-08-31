// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-envparse"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/gen/maps"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/internal/pkg/capability"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	extservices "github.com/siderolabs/talos/pkg/machinery/extensions/services"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/time"
)

// Extension service is a generic wrapper around extension services spec.
type Extension struct {
	Spec extservices.Spec

	overlayUnmounter    func() error
	preShutdownRunnerFn func(bool, *runner.Args, ...runner.Option) runner.Runner
}

// ID implements the Service interface.
func (svc *Extension) ID(r runtime.Runtime) string {
	return "ext-" + svc.Spec.Name
}

// PreFunc implements the Service interface.
func (svc *Extension) PreFunc(ctx context.Context, r runtime.Runtime) error {
	if svc.Spec.RunnerMode == extservices.RunnerModeHost {
		// Host services run binaries installed directly into the Talos host filesystem.
		return nil
	}

	// re-mount service rootfs as overlay rw mount to allow containerd to mount there /dev, /proc, etc.
	rootfsPath := filepath.Join(constants.ExtensionServiceRootfsPath, svc.Spec.Name)

	// TODO: label system extensions
	overlay := mount.NewSystemOverlay(
		[]string{rootfsPath},
		rootfsPath,
		nil,
	)

	if _, err := overlay.Mount(); err != nil {
		return err
	}

	svc.overlayUnmounter = overlay.Unmount

	return nil
}

// PostFunc implements the Service interface.
func (svc *Extension) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	if svc.overlayUnmounter == nil {
		return nil
	}

	return svc.overlayUnmounter()
}

// PreShutdownFunc runs the configured node shutdown hook while the service is still running.
func (svc *Extension) PreShutdownFunc(ctx context.Context, r runtime.Runtime) error {
	if svc.Spec.PreShutdown == nil {
		return nil
	}

	envVars, err := svc.preShutdownEnvironment(ctx, r)
	if err != nil {
		return err
	}

	hookCtx, cancel := context.WithTimeout(ctx, svc.Spec.PreShutdown.Timeout)
	defer cancel()

	hookRunner := svc.preShutdownRunner(r, envVars)
	if err = hookRunner.Open(); err != nil {
		return fmt.Errorf("failed to open pre-shutdown hook runner: %w", err)
	}

	defer hookRunner.Close() //nolint:errcheck

	_, err = hookRunner.Run(hookCtx, events.NullRecorder, nil)

	return svc.preShutdownError(err, hookCtx.Err())
}

func (svc *Extension) preShutdownEnvironment(ctx context.Context, r runtime.Runtime) ([]string, error) {
	envVars, err := svc.parseEnvironment()
	if err != nil {
		return nil, err
	}

	configSpec, err := safe.StateGetByID[*runtimeres.ExtensionServiceConfig](ctx, r.State().V1Alpha2().Resources(), svc.Spec.Name)
	if state.IsNotFoundError(err) {
		return slices.Concat(envVars, environment.Get(r.Config())), nil
	}

	if err != nil {
		return nil, err
	}

	_, envVars, err = svc.applyExtensionServiceConfig(configSpec.TypedSpec(), nil, envVars)
	if err != nil {
		return nil, err
	}

	return slices.Concat(envVars, environment.Get(r.Config())), nil
}

func (svc *Extension) preShutdownRunner(r runtime.Runtime, envVars []string) runner.Runner {
	logToConsole := svc.Spec.LogToConsole
	if r.Config() != nil && r.Config().Debug() {
		logToConsole = true
	}

	runnerFn := svc.preShutdownRunnerFn
	if runnerFn == nil {
		runnerFn = process.NewRunner
	}

	return runnerFn(
		logToConsole,
		&runner.Args{
			ID:          svc.ID(r) + "-pre-shutdown",
			ProcessArgs: append([]string{svc.Spec.PreShutdown.Entrypoint}, svc.Spec.PreShutdown.Args...),
		},
		runner.WithLoggingManager(r.Logging()),
		runner.WithEnv(envVars),
		runner.WithCgroupPath(filepath.Join(constants.CgroupExtensions, svc.Spec.Name)),
		runner.WithGracefulShutdownTimeout(0),
		runner.WithOOMScoreAdj(-600),
	)
}

func (svc *Extension) preShutdownError(runErr, contextErr error) error {
	switch contextErr {
	case context.DeadlineExceeded:
		return fmt.Errorf("pre-shutdown hook timed out after %s: %w", svc.Spec.PreShutdown.Timeout, contextErr)
	case context.Canceled:
		return fmt.Errorf("pre-shutdown hook canceled: %w", contextErr)
	}

	if runErr != nil {
		return fmt.Errorf("pre-shutdown hook failed: %w", runErr)
	}

	return nil
}

// Condition implements the Service interface.
func (svc *Extension) Condition(r runtime.Runtime) conditions.Condition {
	var conds []conditions.Condition

	if svc.Spec.Container.EnvironmentFile != "" {
		// add a dependency on the environment file
		conds = append(conds, conditions.WaitForFileToExist(svc.Spec.Container.EnvironmentFile))
	}

	for _, dep := range svc.Spec.Depends {
		switch {
		case dep.Path != "":
			conds = append(conds, conditions.WaitForFileToExist(dep.Path))
		case len(dep.Network) > 0:
			conds = append(conds, network.NewReadyCondition(r.State().V1Alpha2().Resources(), network.StatusChecksFromStatuses(dep.Network...)...))
		case dep.Time:
			conds = append(conds, time.NewSyncCondition(r.State().V1Alpha2().Resources()))
		case dep.Configuration:
			conds = append(conds, runtimeres.NewExtensionServiceConfigStatusCondition(r.State().V1Alpha2().Resources(), svc.Spec.Name))
		}
	}

	if len(conds) == 0 {
		return nil
	}

	return conditions.WaitForAll(conds...)
}

// DependsOn implements the Service interface.
func (svc *Extension) DependsOn(r runtime.Runtime) []string {
	var deps []string

	if svc.Spec.RunnerMode == extservices.RunnerModeContainer {
		deps = append(deps, "containerd")
	}

	for _, dep := range svc.Spec.Depends {
		if dep.Service != "" {
			deps = append(deps, dep.Service)
		}
	}

	return deps
}

// Volumes implements the Service interface.
func (svc *Extension) Volumes(runtime.Runtime) []string {
	return nil
}

func (svc *Extension) getOCIOptions(envVars []string, mounts []specs.Mount) []oci.SpecOpts {
	ociOpts := []oci.SpecOpts{
		oci.WithRootFSPath(filepath.Join(constants.ExtensionServiceRootfsPath, svc.Spec.Name)),
		containerd.WithRootfsPropagation(svc.Spec.Container.Security.RootfsPropagation),
		oci.WithMounts(mounts),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostNamespace(specs.IPCNamespace),
		oci.WithSelinuxLabel(""),
		oci.WithApparmorProfile(""),
		oci.WithCapabilities(capability.AllGrantableCapabilities()),
		oci.WithAllDevicesAllowed,
		oci.WithEnv(envVars),
		func(_ context.Context, _ oci.Client, _ *containers.Container, spec *oci.Spec) error {
			// clear the rlimits, allow to inherit from machined -> containerd
			// (see https://github.com/containerd/cri/issues/515)
			spec.Process.Rlimits = nil

			return nil
		},
	}

	if !svc.Spec.Container.Security.WriteableRootfs {
		ociOpts = append(ociOpts, oci.WithRootFSReadonly())
	}

	if svc.Spec.Container.Security.WriteableSysfs {
		ociOpts = append(ociOpts, oci.WithWriteableSysfs)
	}

	if svc.Spec.Container.Security.MaskedPaths != nil {
		ociOpts = append(ociOpts, oci.WithMaskedPaths(svc.Spec.Container.Security.MaskedPaths))
	}

	if svc.Spec.Container.Security.ReadonlyPaths != nil {
		ociOpts = append(ociOpts, oci.WithReadonlyPaths(svc.Spec.Container.Security.ReadonlyPaths))
	}

	return ociOpts
}

// Runner implements the Service interface.
//
//nolint:gocyclo
func (svc *Extension) Runner(r runtime.Runtime) (runner.Runner, error) {
	envVars, err := svc.parseEnvironment()
	if err != nil {
		return nil, err
	}

	mounts := append([]specs.Mount{}, svc.Spec.Container.Mounts...)

	configSpec, err := safe.StateGetByID[*runtimeres.ExtensionServiceConfig](context.Background(), r.State().V1Alpha2().Resources(), svc.Spec.Name)
	if err == nil {
		mounts, envVars, err = svc.applyExtensionServiceConfig(configSpec.TypedSpec(), mounts, envVars)
		if err != nil {
			return nil, err
		}
	} else if !state.IsNotFoundError(err) {
		return nil, err
	}

	var restartType restart.Type

	switch svc.Spec.Restart {
	case extservices.RestartAlways:
		restartType = restart.Forever
	case extservices.RestartNever:
		restartType = restart.Once
	case extservices.RestartUntilSuccess:
		restartType = restart.UntilSuccess
	}

	logToConsole := false

	if r.Config() != nil {
		logToConsole = r.Config().Debug()
	}

	if svc.Spec.LogToConsole {
		logToConsole = true
	}

	if svc.Spec.RunnerMode == extservices.RunnerModeHost {
		args, err := svc.hostProcessArgs(r)
		if err != nil {
			return nil, err
		}

		return restart.New(
			process.NewRunner(
				logToConsole,
				&args,
				runner.WithLoggingManager(r.Logging()),
				runner.WithEnv(slices.Concat(envVars, environment.Get(r.Config()))),
				runner.WithCgroupPath(filepath.Join(constants.CgroupExtensions, svc.Spec.Name)),
				runner.WithOOMScoreAdj(-600),
			),
			restart.WithType(restartType),
		), nil
	}

	args := runner.Args{
		ID:          svc.ID(r),
		ProcessArgs: append([]string{svc.Spec.Container.Entrypoint}, svc.Spec.Container.Args...),
	}

	for _, mount := range svc.Spec.Container.Mounts {
		if _, err = os.Stat(mount.Source); err == nil {
			// already exists, skip
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		if err = os.MkdirAll(mount.Source, 0o700); err != nil {
			return nil, err
		}
	}

	mounts = bindMountContainerMarker(mounts)

	ociSpecOpts := svc.getOCIOptions(envVars, mounts)

	return restart.New(
		containerd.NewRunner(
			logToConsole,
			&args,
			runner.WithLoggingManager(r.Logging()),
			runner.WithNamespace(constants.SystemContainerdNamespace),
			runner.WithContainerdAddress(constants.SystemContainerdAddress),
			runner.WithEnv(environment.Get(r.Config())),
			runner.WithOCISpecOpts(ociSpecOpts...),
			runner.WithCgroupPath(filepath.Join(constants.CgroupExtensions, svc.Spec.Name)),
			runner.WithOOMScoreAdj(-600),
		),
		restart.WithType(restartType),
	), nil
}

func (svc *Extension) hostProcessArgs(r runtime.Runtime) (runner.Args, error) {
	if !filepath.IsAbs(svc.Spec.Container.Entrypoint) {
		return runner.Args{}, fmt.Errorf("host runner entrypoint must be an absolute path: %q", svc.Spec.Container.Entrypoint)
	}

	return runner.Args{
		ID:          svc.ID(r),
		ProcessArgs: append([]string{svc.Spec.Container.Entrypoint}, svc.Spec.Container.Args...),
	}, nil
}

func (svc *Extension) applyExtensionServiceConfig(
	spec *runtimeres.ExtensionServiceConfigSpec,
	mounts []specs.Mount,
	envVars []string,
) ([]specs.Mount, []string, error) {
	if svc.Spec.RunnerMode == extservices.RunnerModeHost && len(spec.Files) > 0 {
		return nil, nil, errors.New("extension service config files are not supported in host runner mode")
	}

	for _, ext := range spec.Files {
		mounts = append(mounts, specs.Mount{
			Source:      filepath.Join(constants.ExtensionServiceUserConfigPath, svc.Spec.Name, strings.ReplaceAll(strings.TrimPrefix(ext.MountPath, "/"), "/", "-")),
			Destination: ext.MountPath,
			Type:        "bind",
			Options:     []string{"ro", "bind"},
		})
	}

	return mounts, append(envVars, spec.Environment...), nil
}

// APIRestartAllowed implements APIRestartableService.
func (svc *Extension) APIRestartAllowed(runtime.Runtime) bool {
	return true
}

// APIStartAllowed implements APIStartableService.
func (svc *Extension) APIStartAllowed(runtime.Runtime) bool {
	return true
}

// APIStopAllowed implements APIStoppableService.
func (svc *Extension) APIStopAllowed(runtime.Runtime) bool {
	return true
}

func (svc *Extension) parseEnvironment() ([]string, error) {
	var envVars []string

	if svc.Spec.Container.EnvironmentFile != "" {
		envFile, err := os.OpenFile(svc.Spec.Container.EnvironmentFile, os.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}

		defer func() {
			if closeErr := envFile.Close(); err != nil {
				err = closeErr
			}
		}()

		parsedEnvVars, err := envparse.Parse(envFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse environment file %q: %w", svc.Spec.Container.EnvironmentFile, err)
		}

		envVarsSlice := maps.ToSlice(parsedEnvVars, func(k, v string) string {
			return fmt.Sprintf("%s=%s", k, v)
		})

		envVars = append(envVars, envVarsSlice...)
	}

	if svc.Spec.Container.Environment != nil {
		envVars = append(envVars, svc.Spec.Container.Environment...)
	}

	return envVars, nil
}
