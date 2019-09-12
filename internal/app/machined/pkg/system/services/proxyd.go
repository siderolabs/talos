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

	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Proxyd implements the Service interface. It serves as the concrete type with
// the required methods.
type Proxyd struct{}

// ID implements the Service interface.
func (p *Proxyd) ID(data *userdata.UserData) string {
	return "proxyd"
}

// PreFunc implements the Service interface.
func (p *Proxyd) PreFunc(ctx context.Context, data *userdata.UserData) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))
	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/proxyd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/proxyd"),
		},
	})
}

// PostFunc implements the Service interface.
func (p *Proxyd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (p *Proxyd) Condition(data *userdata.UserData) conditions.Condition {
	return conditions.WaitForFilesToExist("/etc/kubernetes/pki/ca.crt")
}

// DependsOn implements the Service interface.
func (p *Proxyd) DependsOn(data *userdata.UserData) []string {
	return []string{"system-containerd"}
}

func (p *Proxyd) Runner(data *userdata.UserData) (runner.Runner, error) {
	image := "talos/proxyd"

	// Set the process arguments.
	args := runner.Args{
		ID:          p.ID(data),
		ProcessArgs: []string{"/proxyd", "--userdata=" + constants.UserDataPath},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		data,
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
func (p *Proxyd) HealthFunc(*userdata.UserData) health.Check {
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
func (p *Proxyd) HealthSettings(*userdata.UserData) *health.Settings {
	return &health.DefaultSettings
}

// Verify healthchecked interface
var (
	_ system.HealthcheckedService = &Proxyd{}
)
