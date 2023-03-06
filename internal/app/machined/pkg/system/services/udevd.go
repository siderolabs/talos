// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var _ system.HealthcheckedService = (*Udevd)(nil)

// Udevd implements the Service interface. It serves as the concrete type with
// the required methods.
type Udevd struct {
	triggered bool
}

// ID implements the Service interface.
func (c *Udevd) ID(r runtime.Runtime) string {
	return "udevd"
}

// PreFunc implements the Service interface.
func (c *Udevd) PreFunc(ctx context.Context, r runtime.Runtime) error {
	_, err := cmd.RunContext(
		ctx,
		"/sbin/udevadm",
		"hwdb",
		"--update",
	)

	return err
}

// PostFunc implements the Service interface.
func (c *Udevd) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *Udevd) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *Udevd) DependsOn(r runtime.Runtime) []string {
	return nil
}

// Runner implements the Service interface.
func (c *Udevd) Runner(r runtime.Runtime) (runner.Runner, error) {
	debug := false

	if r.Config() != nil {
		debug = r.Config().Debug()
	}

	processArgs := []string{
		"/sbin/udevd",
		"--resolve-names=never",
	}

	if debug {
		processArgs = append(processArgs, "--debug")
	}

	// Set the process arguments.
	args := &runner.Args{
		ID:          c.ID(r),
		ProcessArgs: processArgs,
	}

	return restart.New(process.NewRunner(
		debug,
		args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithCgroupPath(constants.CgroupSystemRuntime),
		runner.WithDroppedCapabilities(constants.UdevdDroppedCapabilities),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (c *Udevd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		// checking for the existence of the udev control socket is a faster way to check
		// that udevd is running, but not a complete check since the socket can persist if the process
		// was not gracefully stopped
		if err := conditions.WaitForFileToExist("/run/udev/control").Wait(ctx); err != nil {
			return err
		}

		// udevadm trigger returns with an exit code of 0 even if udevd is not fully running,
		// so running `udevadm control --start-exec-queue` to ensure that udevd is fully initialized
		// which returns an exit code of 2 if udevd is not running. This complementes the previous check
		if _, err := cmd.RunContext(ctx, "/sbin/udevadm", "control", "--start-exec-queue"); err != nil {
			return err
		}

		if !c.triggered {
			if _, err := cmd.RunContext(ctx, "/sbin/udevadm", "trigger", "--type=devices", "--action=add"); err != nil {
				return err
			}

			if _, err := cmd.RunContext(ctx, "/sbin/udevadm", "trigger", "--type=subsystems", "--action=add"); err != nil {
				return err
			}

			c.triggered = true
		}

		// This ensures that `udevd` finishes processing kernel events, triggered by
		// `udevd trigger`, to prevent a race condition when a user specifies a path
		// under `/dev/disk/*` in any disk definitions.
		_, err := cmd.RunContext(ctx, "/sbin/udevadm", "settle", "--timeout=50") // timeout here should be less than health.Settings.Timeout

		return err
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (c *Udevd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.Settings{
		InitialDelay: 100 * time.Millisecond,
		Period:       time.Minute,
		Timeout:      55 * time.Second,
	}
}
