// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/syndtr/gocapability/capability"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/dialer"
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
func (o *OSD) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (o *OSD) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (o *OSD) DependsOn(config runtime.Configurator) []string {
	if config.Machine().Type() == machine.TypeWorker {
		return []string{"system-containerd", "containerd"}
	}

	return []string{"system-containerd", "containerd"}
}

func (o *OSD) Runner(config runtime.Configurator) (runner.Runner, error) {
	image := "talos/osd"

	// Set the process arguments.
	args := runner.Args{
		ID: o.ID(config),
		ProcessArgs: []string{
			"/osd",
		},
	}

	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.OSSocketPath), 0750); err != nil {
		return nil, err
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: constants.ContainerdAddress, Source: constants.ContainerdAddress, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.DefaultLogPath, Source: constants.DefaultLogPath, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.SystemRunPath, Source: constants.SystemRunPath, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.OSSocketPath), Source: filepath.Dir(constants.OSSocketPath), Options: []string{"rbind", "rw"}},
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
			oci.WithCapabilities([]string{
				strings.ToUpper("CAP_" + capability.CAP_SYS_PTRACE.String()),
				strings.ToUpper("CAP_" + capability.CAP_DAC_READ_SEARCH.String()),
				strings.ToUpper("CAP_" + capability.CAP_DAC_OVERRIDE.String()),
				strings.ToUpper("CAP_" + capability.CAP_SYSLOG.String()),
			}),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithMounts(mounts),
			oci.WithLinuxDevice("/dev/kmsg", "r"),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (o *OSD) HealthFunc(runtime.Configurator) health.Check {
	return func(ctx context.Context) error {
		conn, err := grpc.Dial(
			fmt.Sprintf("%s://%s", "unix", constants.OSSocketPath),
			grpc.WithInsecure(),
			grpc.WithContextDialer(dialer.DialUnix()),
		)
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
