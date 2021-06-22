// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint
package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/oci"
	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/go-debug"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/secrets"
)

// APID implements the Service interface. It serves as the concrete type with
// the required methods.
type APID struct {
	runtimeServer *grpc.Server
}

// ID implements the Service interface.
func (o *APID) ID(r runtime.Runtime) string {
	return "apid"
}

// PreFunc implements the Service interface.
func (o *APID) PreFunc(ctx context.Context, r runtime.Runtime) error {
	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.APIRuntimeSocketPath), 0o750); err != nil {
		return err
	}

	listener, err := net.Listen("unix", constants.APIRuntimeSocketPath)
	if err != nil {
		return err
	}

	// TODO: filter State to return only resources apid needs
	o.runtimeServer = grpc.NewServer()
	v1alpha1.RegisterStateServer(o.runtimeServer, server.NewState(r.State().V1Alpha2().Resources()))

	go o.runtimeServer.Serve(listener) //nolint:errcheck

	return prepareRootfs(o.ID(r))
}

// PostFunc implements the Service interface.
func (o *APID) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	o.runtimeServer.Stop()

	return os.RemoveAll(constants.APIRuntimeSocketPath)
}

// Condition implements the Service interface.
func (o *APID) Condition(r runtime.Runtime) conditions.Condition {
	return secrets.NewAPIReadyCondition(r.State().V1Alpha2().Resources())
}

// DependsOn implements the Service interface.
func (o *APID) DependsOn(r runtime.Runtime) []string {
	return []string{"containerd"}
}

// Runner implements the Service interface.
func (o *APID) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.APISocketPath), 0o750); err != nil {
		return nil, err
	}

	// Set the process arguments.
	args := runner.Args{
		ID: o.ID(r),
		ProcessArgs: []string{
			"/apid",
		},
	}

	if r.Config().Machine().Features().RBACEnabled() {
		args.ProcessArgs = append(args.ProcessArgs, "--enable-rbac")
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.MachineSocketPath), Source: filepath.Dir(constants.MachineSocketPath), Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.APISocketPath), Source: filepath.Dir(constants.APISocketPath), Options: []string{"rbind", "rw"}},
	}

	env := []string{}

	for key, val := range r.Config().Machine().Env() {
		switch strings.ToLower(key) {
		// explicitly exclude proxy variables from apid since this will
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

	if debug.RaceEnabled {
		env = append(env, "GORACE=halt_on_error=1")
	}

	return restart.New(containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
			oci.WithRootFSPath(filepath.Join(constants.SystemLibexecPath, o.ID(r))),
			oci.WithRootFSReadonly(),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (o *APID) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer

		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", "127.0.0.1", constants.ApidPort))
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (o *APID) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
