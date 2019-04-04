/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"fmt"

	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/process"
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
func (c *Udevd) PreFunc(data *userdata.UserData) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *Udevd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (c *Udevd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

// Start implements the Service interface.
func (c *Udevd) Start(data *userdata.UserData) error {
	// Set the process arguments.
	args := &runner.Args{
		ID:          c.ID(data),
		ProcessArgs: []string{"/bin/udevd"},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	r := process.Process{}

	return r.Run(
		data,
		args,
		runner.WithEnv(env),
	)
}
