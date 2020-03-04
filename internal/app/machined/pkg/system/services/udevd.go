// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/cmd"
)

// Udevd implements the Service interface. It serves as the concrete type with
// the required methods.
type Udevd struct{}

// ID implements the Service interface.
func (c *Udevd) ID(config runtime.Configurator) string {
	return "udevd"
}

// PreFunc implements the Service interface.
func (c *Udevd) PreFunc(ctx context.Context, config runtime.Configurator) error {
	_, err := cmd.Run(
		"/sbin/udevadm",
		"hwdb",
		"--update",
	)

	return err
}

// PostFunc implements the Service interface.
func (c *Udevd) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *Udevd) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *Udevd) DependsOn(config runtime.Configurator) []string {
	return nil
}

// Runner implements the Service interface.
func (c *Udevd) Runner(config runtime.Configurator) (runner.Runner, error) {
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(config),
		ProcessArgs: []string{
			"/sbin/udevd",
			"--resolve-names=never",
			"-D",
		},
	}

	env := []string{}
	for key, val := range config.Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(process.NewRunner(
		config.Debug(),
		args,
		runner.WithEnv(env),
	),
		restart.WithType(restart.Forever),
	), nil
}
