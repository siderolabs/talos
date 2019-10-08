/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/retry"
)

// OSD implements the Service interface. It serves as the concrete type with
// the required methods.
type OSD struct{}

// ID implements the Service interface.
func (o *OSD) ID(config runtime.Configurator) string {
	return "osd"
}

// PreFunc implements the Service interface.
func (o *OSD) PreFunc(ctx context.Context, config runtime.Configurator) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))

	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/osd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/osd"),
		},
	})
}

// PostFunc implements the Service interface.
func (o *OSD) PostFunc(config runtime.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (o *OSD) Condition(config runtime.Configurator) conditions.Condition {
	if config.Machine().Type() == machine.Worker {
		return conditions.WaitForFileToExist(constants.KubeletKubeconfig)
	}

	return nil
}

// DependsOn implements the Service interface.
func (o *OSD) DependsOn(config runtime.Configurator) []string {
	if config.Machine().Type() == machine.Worker {
		return []string{"system-containerd", "containerd", "kubelet"}
	}

	return []string{"system-containerd", "containerd"}
}

func (o *OSD) Runner(config runtime.Configurator) (runner.Runner, error) {
	image := "talos/osd"

	endpoints := []string{"127.0.0.1"}

	if config.Machine().Type() == machine.Worker {
		opts := []retry.Option{retry.WithUnits(3 * time.Second), retry.WithJitter(time.Second)}

		err := retry.Constant(10*time.Minute, opts...).Retry(func() error {
			h, err := kubernetes.NewHelper()
			if err != nil {
				return retry.ExpectedError(errors.Wrap(err, "failed to create client"))
			}

			endpoints, err = h.MasterIPs()
			if err != nil {
				return retry.ExpectedError(err)
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Set the process arguments.
	args := runner.Args{
		ID: o.ID(config),
		ProcessArgs: []string{
			"/osd",
			"--config=" + constants.ConfigPath,
			"--endpoints=" + strings.Join(endpoints, ","),
		},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: constants.ContainerdAddress, Source: constants.ContainerdAddress, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.SystemRunPath, Source: constants.SystemRunPath, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/var/log/pods", Source: "/var/log/pods", Options: []string{"bind", "ro"}},
	}

	env := []string{}
	for key, val := range config.Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		config.Debug(),
		&args,
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (o *OSD) HealthFunc(runtime.Configurator) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", "127.0.0.1", constants.OsdPort))
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface
func (o *OSD) HealthSettings(runtime.Configurator) *health.Settings {
	return &health.DefaultSettings
}
