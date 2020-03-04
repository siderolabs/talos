// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"
	"net"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Trustd implements the Service interface. It serves as the concrete type with
// the required methods.
type Trustd struct{}

// ID implements the Service interface.
func (t *Trustd) ID(config runtime.Configurator) string {
	return "trustd"
}

// PreFunc implements the Service interface.
func (t *Trustd) PreFunc(ctx context.Context, config runtime.Configurator) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))

	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/trustd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/trustd"),
		},
	})
}

// PostFunc implements the Service interface.
func (t *Trustd) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (t *Trustd) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (t *Trustd) DependsOn(config runtime.Configurator) []string {
	return []string{"containerd"}
}

func (t *Trustd) Runner(config runtime.Configurator) (runner.Runner, error) {
	image := "talos/trustd"

	// Set the process arguments.
	args := runner.Args{
		ID: t.ID(config),
		ProcessArgs: []string{
			"/trustd",
			"--config=" + constants.ConfigPath,
		},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"rbind", "ro"}},
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
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (t *Trustd) HealthFunc(runtime.Configurator) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer

		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", "127.0.0.1", constants.TrustdPort))
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface
func (t *Trustd) HealthSettings(runtime.Configurator) *health.Settings {
	return &health.DefaultSettings
}
