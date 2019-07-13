/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Udevd implements the Service interface. It serves as the concrete type with
// the required methods.
type Udevd struct{}

// ID implements the Service interface.
func (c *Udevd) ID(data *userdata.UserData) string {
	return "udevd"
}

// PreFunc implements the Service interface.
func (c *Udevd) PreFunc(ctx context.Context, data *userdata.UserData) error {
	cmd := exec.Command(
		"/sbin/udevadm",
		"hwdb",
		"--update",
	)
	return cmd.Run()
}

// PostFunc implements the Service interface.
func (c *Udevd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *Udevd) Condition(data *userdata.UserData) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *Udevd) DependsOn(data *userdata.UserData) []string {
	return nil
}

// Runner implements the Service interface.
func (c *Udevd) Runner(data *userdata.UserData) (runner.Runner, error) {
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(data),
		ProcessArgs: []string{
			"/sbin/udevd",
			"--resolve-names=never",
			"-D",
		},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(process.NewRunner(
		data,
		args,
		runner.WithEnv(env),
	),
		restart.WithType(restart.Forever),
	), nil
}
