// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	"path/filepath"

	containerd "github.com/containerd/containerd/v2/client"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var _ system.HealthcheckedService = (*Containerd)(nil)

// Containerd implements the Service interface. It serves as the concrete type with
// the required methods.
type Containerd struct {
	// client is a lazy-initialized containerd client. It should be accessed using the Client() method.
	client *containerd.Client
}

// Client lazy-initializes the containerd client if needed and returns it.
func (c *Containerd) Client() (*containerd.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return nil, err
	}

	c.client = client

	return c.client, err
}

// ID implements the Service interface.
func (c *Containerd) ID(runtime.Runtime) string {
	return "containerd"
}

// PreFunc implements the Service interface.
func (c *Containerd) PreFunc(context.Context, runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *Containerd) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
	if c.client != nil {
		return c.client.Close()
	}

	return nil
}

// Condition implements the Service interface.
func (c *Containerd) Condition(runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *Containerd) DependsOn(runtime.Runtime) []string {
	return nil
}

// Volumes implements the Service interface.
func (c *Containerd) Volumes() []string {
	return nil
}

// Runner implements the Service interface.
func (c *Containerd) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(r),
		ProcessArgs: []string{
			"/bin/containerd",
			"--address",
			constants.SystemContainerdAddress,
			"--state",
			filepath.Join(constants.SystemRunPath, "containerd"),
			"--root",
			filepath.Join(constants.SystemVarPath, "lib", "containerd"),
		},
	}

	debug := false

	if r.Config() != nil {
		debug = r.Config().Debug()
	}

	return restart.New(process.NewRunner(
		debug,
		args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithEnv(append(
			environment.Get(r.Config()),
			// append a default value for XDG_RUNTIME_DIR for the services running on the host
			// see https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
			"XDG_RUNTIME_DIR=/run",
		)),
		runner.WithOOMScoreAdj(-999),
		runner.WithCgroupPath(constants.CgroupSystemRuntime),
		runner.WithSelinuxLabel(constants.SelinuxLabelSystemRuntime),
		runner.WithDroppedCapabilities(constants.DefaultDroppedCapabilities),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (c *Containerd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		client, err := c.Client()
		if err != nil {
			return err
		}

		resp, err := client.HealthService().Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil {
			return err
		}

		if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			return fmt.Errorf("unexpected serving status: %d", resp.Status)
		}

		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (c *Containerd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
