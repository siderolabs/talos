/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/pkg/errors"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/constants"
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
func (c *Containerd) PreFunc(ctx context.Context, data *userdata.UserData) error {
	return os.MkdirAll(defaults.DefaultRootDir, os.ModeDir)
}

// PostFunc implements the Service interface.
func (c *Containerd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *Containerd) Condition(data *userdata.UserData) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *Containerd) DependsOn(data *userdata.UserData) []string {
	return nil
}

// Runner implements the Service interface.
func (c *Containerd) Runner(data *userdata.UserData) (runner.Runner, error) {
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(data),
		ProcessArgs: []string{"/bin/containerd",
			"--address",
			constants.ContainerdAddress,
			"--config",
			"/etc/containerd.toml",
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

// HealthFunc implements the HealthcheckedService interface
func (c *Containerd) HealthFunc(*userdata.UserData) health.Check {
	return func(ctx context.Context) error {
		client, err := containerd.New(constants.ContainerdAddress)
		if err != nil {
			return err
		}
		// nolint: errcheck
		defer client.Close()

		resp, err := client.HealthService().Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil {
			return err
		}

		if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			return errors.Errorf("unexpected serving status: %d", resp.Status)
		}

		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface
func (c *Containerd) HealthSettings(*userdata.UserData) *health.Settings {
	return &health.DefaultSettings
}

// Verify healthchecked interface
var (
	_ system.HealthcheckedService = &Containerd{}
)
