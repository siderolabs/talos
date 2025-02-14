// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	"os"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/defaults"
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
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

var _ system.HealthcheckedService = (*CRI)(nil)

// CRI implements the Service interface. It serves as the concrete type with
// the required methods.
type CRI struct {
	// client is a lazy-initialized containerd client. It should be accessed using the Client() method.
	client *containerd.Client
}

// Client lazy-initializes the containerd client if needed and returns it.
func (c *CRI) Client() (*containerd.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	client, err := containerd.New(constants.CRIContainerdAddress)
	if err != nil {
		return nil, err
	}

	c.client = client

	return c.client, err
}

// ID implements the Service interface.
func (c *CRI) ID(runtime.Runtime) string {
	return "cri"
}

// PreFunc implements the Service interface.
func (c *CRI) PreFunc(context.Context, runtime.Runtime) error {
	return os.MkdirAll(defaults.DefaultRootDir, os.ModeDir)
}

// PostFunc implements the Service interface.
func (c *CRI) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
	if c.client != nil {
		return c.client.Close()
	}

	return nil
}

// Condition implements the Service interface.
func (c *CRI) Condition(r runtime.Runtime) conditions.Condition {
	return network.NewReadyCondition(r.State().V1Alpha2().Resources(), network.AddressReady, network.HostnameReady, network.EtcFilesReady)
}

// DependsOn implements the Service interface.
func (c *CRI) DependsOn(runtime.Runtime) []string {
	return nil
}

// Volumes implements the Service interface.
func (c *CRI) Volumes() []string {
	return []string{constants.EphemeralPartitionLabel}
}

// Runner implements the Service interface.
func (c *CRI) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(r),
		ProcessArgs: []string{
			"/bin/containerd",
			"--address",
			constants.CRIContainerdAddress,
			"--config",
			constants.CRIContainerdConfig,
		},
	}

	return restart.New(process.NewRunner(
		r.Config().Debug(),
		args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithEnv(append(
			environment.Get(r.Config()),
			// append a default value for XDG_RUNTIME_DIR for the services running on the host
			// see https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
			"XDG_RUNTIME_DIR=/run",
		)),
		runner.WithOOMScoreAdj(-500),
		runner.WithCgroupPath(constants.CgroupPodRuntime),
		runner.WithSelinuxLabel(constants.SelinuxLabelPodRuntime),
		runner.WithDroppedCapabilities(constants.DefaultDroppedCapabilities),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (c *CRI) HealthFunc(runtime.Runtime) health.Check {
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
func (c *CRI) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
