/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"fmt"
	"os"

	"github.com/containerd/containerd/defaults"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/process"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Containerd implements the Service interface. It serves as the concrete type with
// the required methods.
type Containerd struct{}

// ID implements the Service interface.
func (c *Containerd) ID(data *userdata.UserData) string {
	return "containerd"
}

// PreFunc implements the Service interface.
func (c *Containerd) PreFunc(data *userdata.UserData) error {
	return os.MkdirAll(defaults.DefaultRootDir, os.ModeDir)
}

// PostFunc implements the Service interface.
func (c *Containerd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (c *Containerd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

// Start implements the Service interface.
func (c *Containerd) Start(data *userdata.UserData) error {
	// Set the process arguments.
	args := &runner.Args{
		ID:          c.ID(data),
		ProcessArgs: []string{"/bin/containerd"},
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
