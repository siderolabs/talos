/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"
	"net"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

// Proxyd implements the Service interface. It serves as the concrete type with
// the required methods.
type Proxyd struct{}

// ID implements the Service interface.
func (p *Proxyd) ID(config config.Configurator) string {
	return "proxyd"
}

// PreFunc implements the Service interface.
func (p *Proxyd) PreFunc(ctx context.Context, config config.Configurator) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))
	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/proxyd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/proxyd"),
		},
	})
}

// PostFunc implements the Service interface.
func (p *Proxyd) PostFunc(config config.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (p *Proxyd) Condition(config config.Configurator) conditions.Condition {
	return conditions.WaitForFilesToExist(constants.AdminKubeconfig)
}

// DependsOn implements the Service interface.
func (p *Proxyd) DependsOn(config config.Configurator) []string {
	return []string{"system-containerd"}
}

func (p *Proxyd) Runner(config config.Configurator) (runner.Runner, error) {
	image := "talos/proxyd"

	// Set the process arguments.
	args := runner.Args{
		ID:          p.ID(config),
		ProcessArgs: []string{"/proxyd", "--config=" + constants.ConfigPath},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
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
			containerd.WithMemoryLimit(int64(1000000*512)),
			oci.WithMounts(mounts),
			oci.WithPrivileged,
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (p *Proxyd) HealthFunc(config.Configurator) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", "127.0.0.1", 443))
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface
func (p *Proxyd) HealthSettings(config.Configurator) *health.Settings {
	return &health.DefaultSettings
}

// APIStartAllowed implements the APIStartableService interface.
func (p *Proxyd) APIStartAllowed(config config.Configurator) bool {
	return true
}

// APIRestartAllowed implements the APIRestartableService interface.
func (p *Proxyd) APIRestartAllowed(config config.Configurator) bool {
	return true
}
