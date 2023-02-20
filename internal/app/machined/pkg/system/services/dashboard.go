// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint,dupl
package services

import (
	"context"
	"path/filepath"

	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Dashboard implements the Service interface. It serves as the concrete type with
// the required methods.
type Dashboard struct{}

// ID implements the Service interface.
func (d *Dashboard) ID(r runtime.Runtime) string {
	return "dashboard"
}

// PreFunc implements the Service interface.
//
//nolint:gocyclo
func (d *Dashboard) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return prepareRootfs(d.ID(r))
}

// PostFunc implements the Service interface.
func (d *Dashboard) PostFunc(r runtime.Runtime, state events.ServiceState) error {
	return nil
}

// Condition implements the Service interface.
func (d *Dashboard) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (d *Dashboard) DependsOn(r runtime.Runtime) []string {
	return []string{"machined"}
}

// Runner implements the Service interface.
func (d *Dashboard) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Set the process arguments.
	args := runner.Args{
		ID:          d.ID(r),
		ProcessArgs: []string{"/dashboard"},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: filepath.Dir(constants.MachineSocketPath), Source: filepath.Dir(constants.MachineSocketPath), Options: []string{"rbind", "ro"}},
	}

	return restart.New(containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithNamespace(constants.SystemContainerdNamespace),
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithOCISpecOpts(
			oci.WithEnv([]string{"TERM=linux"}),
			oci.WithRootFSPath(filepath.Join(constants.SystemLibexecPath, d.ID(r))),
			oci.WithMounts(mounts),
			oci.WithLinuxDevice("/dev/tty5", "rwm"),
		),
		runner.WithOOMScoreAdj(-400),
	),
		restart.WithType(restart.Forever),
	), nil
}
