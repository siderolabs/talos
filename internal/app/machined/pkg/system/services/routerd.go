// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: golint
package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/grpc/dialer"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Routerd implements the Service interface. It serves as the concrete type with
// the required methods.
type Routerd struct{}

// ID implements the Service interface.
func (o *Routerd) ID(r runtime.Runtime) string {
	return "routerd"
}

// PreFunc implements the Service interface.
func (o *Routerd) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return image.Import(ctx, "/usr/images/routerd.tar", "talos/routerd")
}

// PostFunc implements the Service interface.
func (o *Routerd) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (o *Routerd) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (o *Routerd) DependsOn(r runtime.Runtime) []string {
	return []string{"containerd"}
}

func (o *Routerd) Runner(r runtime.Runtime) (runner.Runner, error) {
	image := "talos/routerd"

	// Set the process arguments.
	args := runner.Args{
		ID: o.ID(r),
		ProcessArgs: []string{
			"/routerd",
		},
	}

	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.RouterdSocketPath), 0o750); err != nil {
		return nil, err
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: constants.SystemRunPath, Source: constants.SystemRunPath, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.RouterdSocketPath), Source: filepath.Dir(constants.RouterdSocketPath), Options: []string{"rbind", "rw"}},
	}

	env := []string{}

	for key, val := range r.Config().Machine().Env() {
		switch strings.ToLower(key) {
		// explicitly exclude proxy variables from routerd since this will
		// negatively impact grpc connections.
		// ref: https://github.com/grpc/grpc-go/blob/0f32486dd3c9bc29705535bd7e2e43801824cbc4/clientconn.go#L199-L206
		// ref: https://github.com/grpc/grpc-go/blob/63ae68c9686cc0dd26c4f7476d66bb2f5c31789f/proxy.go#L118-L144
		case "no_proxy":
		case "http_proxy":
		case "https_proxy":
		default:
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	return restart.New(containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithLoggingManager(r.Logging()),
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

// HealthFunc implements the HealthcheckedService interface.
func (o *Routerd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		conn, err := grpc.DialContext(
			ctx,
			fmt.Sprintf("%s://%s", "unix", constants.RouterdSocketPath),
			grpc.WithInsecure(),
			grpc.WithContextDialer(dialer.DialUnix()),
		)
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (o *Routerd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
