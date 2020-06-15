// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
)

// Iscsid implements the Service interface. It serves as the concrete type with
// the required methods.
type Iscsid struct{}

// ID implements the Service interface.
func (c *Iscsid) ID(r runtime.Runtime) string {
	return "iscsid"
}

// PreFunc implements the Service interface.
func (c *Iscsid) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *Iscsid) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *Iscsid) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *Iscsid) DependsOn(r runtime.Runtime) []string {
	return nil
}

// Runner implements the Service interface.
func (c *Iscsid) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(r),
		ProcessArgs: []string{
			"/sbin/iscsid",
			"-f",
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
