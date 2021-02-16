// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	"time"

	"github.com/talos-systems/go-cmd/pkg/cmd"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/conditions"
)

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
	_, err := cmd.Run(
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
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(r),
		ProcessArgs: []string{
			"/sbin/udevd",
			"--resolve-names=never",
		},
	}

	env := []string{}
	for key, val := range r.Config().Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(process.NewRunner(
		r.Config().Debug(),
		args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithEnv(env),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (c *Udevd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		if !c.triggered {
			if _, err := cmd.RunContext(ctx, "/sbin/udevadm", "trigger"); err != nil {
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
