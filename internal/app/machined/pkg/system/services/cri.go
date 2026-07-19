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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/sandboxd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

var _ system.HealthcheckedService = (*CRI)(nil)

// CRI implements the Service interface. It serves as the concrete type with
// the required methods.
type CRI struct {
	// client is a lazy-initialized containerd client. It should be accessed using the Client() method.
	client *containerd.Client
}

const criServiceID = "cri"

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
	return criServiceID
}

// PreFunc implements the Service interface.
func (c *CRI) PreFunc(ctx context.Context, r runtime.Runtime) error {
	if err := os.MkdirAll(defaults.DefaultRootDir, 0o750); err != nil {
		return err
	}

	if r.State().Platform().Mode().InContainer() {
		return nil
	}

	return createOverlayMountRequests(ctx, r.State().V1Alpha2().Resources())
}

func createOverlayMountRequests(ctx context.Context, st state.State) error {
	for _, overlay := range constants.Overlays {
		mountRequest := block.NewVolumeMountRequest(block.NamespaceName, overlay.Path)
		mountRequest.TypedSpec().Requester = criServiceID
		mountRequest.TypedSpec().VolumeID = overlay.Path

		if err := st.Create(ctx, mountRequest); err != nil && !state.IsConflictError(err) {
			return fmt.Errorf("error creating persistent volume mount request for overlay %q: %w", overlay.Path, err)
		}
	}

	return nil
}

// PostFunc implements the Service interface.
func (c *CRI) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
	if c.client != nil {
		client := c.client
		c.client = nil

		return client.Close()
	}

	return nil
}

// Condition implements the Service interface.
func (c *CRI) Condition(r runtime.Runtime) conditions.Condition {
	cond := []conditions.Condition{
		network.NewReadyCondition(r.State().V1Alpha2().Resources(), network.AddressReady, network.HostnameReady, network.EtcFilesReady),
		files.NewEtcFileCondition(r.State().V1Alpha2().Resources(), "machine-id", constants.CRIConfig, constants.CRIBaseRuntimeSpec),
	}

	cfg := r.Config()

	if !r.State().Platform().Mode().InContainer() && cfg != nil && cfg.UnattendedInstallConfig() != nil {
		cond = append(
			cond,
			runtimeres.NewUnattendedInstallCondition(r.State().V1Alpha2().Resources()),
		)
	}

	return conditions.WaitForAll(cond...)
}

// DependsOn implements the Service interface.
func (c *CRI) DependsOn(r runtime.Runtime) []string {
	if !sandboxd.Enabled(r) {
		return nil
	}

	// CRI runs inside the sandbox namespace, which the sandboxd service owns.
	return []string{sandboxd.ServiceID}
}

// Volumes implements the Service interface.
func (c *CRI) Volumes(r runtime.Runtime) []string {
	volumes := []string{
		"/var/lib",
		"/var/lib/cni",
		constants.CRIContainerdVolumeID,
		"/var/run",
		"/var/run/lock",
	}

	if !r.State().Platform().Mode().InContainer() {
		volumes = append(
			volumes,
			xslices.Map(constants.Overlays, func(target constants.SELinuxLabeledPath) string {
				return target.Path
			})...,
		)
	}

	return volumes
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

	opts := []runner.Option{
		runner.WithLoggingManager(r.Logging()),
		runner.WithEnv(append(
			environment.Get(r.Config()),
			constants.EnvXDGRuntimeDir,
		)),
		runner.WithOOMScoreAdj(-500),
		runner.WithCgroupPath(constants.CgroupPodRuntime),
		runner.WithSelinuxLabel(constants.SelinuxLabelPodRuntime),
		runner.WithDroppedCapabilities(constants.DefaultDroppedCapabilities),
	}

	// When workload isolation is enabled, run CRI inside the shared sandbox
	// PID+mount namespace. The launcher is resolved per launch (getter), so if
	// sandboxd is recreated, CRI's restart re-enters the new namespace. When
	// isolation is disabled/absent (or in container mode) CRI runs on the host.
	if sandboxd.Enabled(r) {
		opts = append(opts, runner.WithSandbox(r.Sandbox))
	}

	return restart.New(
		process.NewRunner(r.Config().Debug(), args, opts...),
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
