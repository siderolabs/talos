/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/talos/internal/app/init/pkg/system"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// OSD implements the Service interface. It serves as the concrete type with
// the required methods.
type OSD struct{}

// ID implements the Service interface.
func (o *OSD) ID(data *userdata.UserData) string {
	return "osd"
}

// PreFunc implements the Service interface.
func (o *OSD) PreFunc(data *userdata.UserData) error {
	return os.MkdirAll("/etc/kubernetes", os.ModeDir)
}

// PostFunc implements the Service interface.
func (o *OSD) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (o *OSD) Condition(data *userdata.UserData) conditions.Condition {
	return conditions.None()
}

func (o *OSD) Runner(data *userdata.UserData) (runner.Runner, error) {
	image := "talos/osd"

	// Set the process arguments.
	args := runner.Args{
		ID:          o.ID(data),
		ProcessArgs: []string{"/osd", "--userdata=" + constants.UserDataPath},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/var/run", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: "/run", Source: "/run", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: constants.ContainerdAddress, Source: constants.ContainerdAddress, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/var/log", Source: "/var/log", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: "/var/lib/init", Source: "/var/lib/init", Options: []string{"rbind", "rw"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		data,
		&args,
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
func (o *OSD) HealthFunc(*userdata.UserData) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", "localhost", constants.OsdPort))
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface
func (o *OSD) HealthSettings(*userdata.UserData) *health.Settings {
	return &health.DefaultSettings
}

// Verify healthchecked interface
var (
	_ system.HealthcheckedService = &OSD{}
)
