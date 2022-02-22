// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/capability"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	extservices "github.com/talos-systems/talos/pkg/machinery/extensions/services"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/machinery/resources/time"
)

// Extension service is a generic wrapper around extension services spec.
type Extension struct {
	Spec *extservices.Spec

	overlay *mount.Point
}

// ID implements the Service interface.
func (svc *Extension) ID(r runtime.Runtime) string {
	return "ext-" + svc.Spec.Name
}

// PreFunc implements the Service interface.
func (svc *Extension) PreFunc(ctx context.Context, r runtime.Runtime) error {
	// re-mount service rootfs as overlay rw mount to allow containerd to mount there /dev, /proc, etc.
	svc.overlay = mount.NewMountPoint(
		"",
		filepath.Join(constants.ExtensionServicesRootfsPath, svc.Spec.Name),
		"",
		0,
		"",
		mount.WithFlags(mount.Overlay|mount.SystemOverlay),
	)

	return svc.overlay.Mount()
}

// PostFunc implements the Service interface.
func (svc *Extension) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return svc.overlay.Unmount()
}

// Condition implements the Service interface.
func (svc *Extension) Condition(r runtime.Runtime) conditions.Condition {
	conds := []conditions.Condition{}

	for _, dep := range svc.Spec.Depends {
		switch {
		case dep.Service != "":
			conds = append(conds, system.WaitForService(system.StateEventUp, dep.Service))
		case dep.Path != "":
			conds = append(conds, conditions.WaitForFileToExist(dep.Path))
		case len(dep.Network) > 0:
			conds = append(conds, network.NewReadyCondition(r.State().V1Alpha2().Resources(), network.StatusChecksFromStatuses(dep.Network...)...))
		case dep.Time:
			conds = append(conds, time.NewSyncCondition(r.State().V1Alpha2().Resources()))
		}
	}

	if len(conds) == 0 {
		return nil
	}

	return conditions.WaitForAll(conds...)
}

// DependsOn implements the Service interface.
func (svc *Extension) DependsOn(r runtime.Runtime) []string {
	return []string{"containerd"}
}

// Runner implements the Service interface.
func (svc *Extension) Runner(r runtime.Runtime) (runner.Runner, error) {
	args := runner.Args{
		ID:          svc.ID(r),
		ProcessArgs: append([]string{svc.Spec.Container.Entrypoint}, svc.Spec.Container.Args...),
	}

	for _, mount := range svc.Spec.Container.Mounts {
		if err := os.MkdirAll(mount.Source, 0o700); err != nil {
			return nil, err
		}
	}

	env := []string{}
	for key, val := range r.Config().Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
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

	return restart.New(containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithNamespace(constants.SystemContainerdNamespace),
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithRootFSPath(filepath.Join(constants.ExtensionServicesRootfsPath, svc.Spec.Name)),
			oci.WithRootFSReadonly(),
			oci.WithCgroup(constants.CgroupExtensions),
			oci.WithMounts(svc.Spec.Container.Mounts),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithSelinuxLabel(""),
			oci.WithApparmorProfile(""),
			oci.WithCapabilities(capability.AllGrantableCapabilities()),
			oci.WithAllDevicesAllowed,
		),
		runner.WithOOMScoreAdj(-600),
	),
		restart.WithType(restartType),
	), nil
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
