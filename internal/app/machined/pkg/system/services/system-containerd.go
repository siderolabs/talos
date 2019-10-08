/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/pkg/errors"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

// SystemContainerd implements the Service interface. It serves as the concrete type with
// the required methods.
type SystemContainerd struct{}

// ID implements the Service interface.
func (c *SystemContainerd) ID(config config.Configurator) string {
	return "system-containerd"
}

// PreFunc implements the Service interface.
func (c *SystemContainerd) PreFunc(ctx context.Context, config config.Configurator) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *SystemContainerd) PostFunc(config config.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *SystemContainerd) Condition(config config.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *SystemContainerd) DependsOn(config config.Configurator) []string {
	return nil
}

// Runner implements the Service interface.
func (c *SystemContainerd) Runner(config config.Configurator) (runner.Runner, error) {
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(config),
		ProcessArgs: []string{
			"/bin/containerd",
			"--address", constants.SystemContainerdAddress,
			"--state", "/run/system/containerd",
			"--root", "/run/system/lib/containerd",
			"--config",
			"/etc/containerd-system.toml",
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

// HealthFunc implements the HealthcheckedService interface
func (c *SystemContainerd) HealthFunc(config.Configurator) health.Check {
	return func(ctx context.Context) error {
		client, err := containerd.New(constants.SystemContainerdAddress)
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
func (c *SystemContainerd) HealthSettings(config.Configurator) *health.Settings {
	return &health.DefaultSettings
}
