// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint
package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/v2/pkg/cap"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/go-debug"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

var _ system.HealthcheckedService = (*APID)(nil)

// APID implements the Service interface. It serves as the concrete type with
// the required methods.
type APID struct {
	runtimeServer *grpc.Server
}

// ID implements the Service interface.
func (o *APID) ID(r runtime.Runtime) string {
	return "apid"
}

// apidResourceFilter filters access to COSI state for apid.
func apidResourceFilter(ctx context.Context, access state.Access) error {
	if !access.Verb.Readonly() {
		return errors.New("write access denied")
	}

	switch {
	case access.ResourceNamespace == secrets.NamespaceName && access.ResourceType == secrets.APIType && access.ResourceID == secrets.APIID:
		// allowed, contains apid certificates
	case access.ResourceNamespace == network.NamespaceName && access.ResourceType == network.NodeAddressType:
		// allowed, contains local node addresses
	case access.ResourceNamespace == network.NamespaceName && access.ResourceType == network.HostnameStatusType:
		// allowed, contains local node hostname
	default:
		return errors.New("access denied")
	}

	return nil
}

// PreFunc implements the Service interface.
func (o *APID) PreFunc(ctx context.Context, r runtime.Runtime) error {
	// filter apid access to make sure apid can only access its certificates
	resources := state.Filter(r.State().V1Alpha2().Resources(), apidResourceFilter)

	// ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.APIRuntimeSocketPath), 0o750); err != nil {
		return err
	}

	// set the final leaf to be world-executable to make apid connect to the socket
	if err := os.Chmod(filepath.Dir(constants.APIRuntimeSocketPath), 0o751); err != nil {
		return err
	}

	// clean up the socket if it already exists (important for Talos in a container)
	if err := os.RemoveAll(constants.APIRuntimeSocketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", constants.APIRuntimeSocketPath)
	if err != nil {
		return err
	}

	// chown the socket path to make it accessible to the apid
	if err := os.Chown(constants.APIRuntimeSocketPath, constants.ApidUserID, constants.ApidUserID); err != nil {
		return err
	}

	o.runtimeServer = grpc.NewServer(
		grpc.SharedWriteBuffer(true),
	)
	v1alpha1.RegisterStateServer(o.runtimeServer, server.NewState(resources))

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

	// Make sure apid user owns socket directory.
	if err := os.Chown(filepath.Dir(constants.APISocketPath), constants.ApidUserID, constants.ApidUserID); err != nil {
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

	if r.Config().Machine().Features().ApidCheckExtKeyUsageEnabled() {
		args.ProcessArgs = append(args.ProcessArgs, "--enable-ext-key-usage-check")
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.MachineSocketPath), Source: filepath.Dir(constants.MachineSocketPath), Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.APISocketPath), Source: filepath.Dir(constants.APISocketPath), Options: []string{"rbind", "rw"}},
	}

	env := []string{
		constants.TcellMinimizeEnvironment,
	}

	for _, value := range environment.Get(r.Config()) {
		key, _, _ := strings.Cut(value, "=")

		switch strings.ToLower(key) {
		// explicitly exclude proxy variables from apid since this will
		// negatively impact grpc connections.
		// ref: https://github.com/grpc/grpc-go/blob/0f32486dd3c9bc29705535bd7e2e43801824cbc4/clientconn.go#L199-L206
		// ref: https://github.com/grpc/grpc-go/blob/63ae68c9686cc0dd26c4f7476d66bb2f5c31789f/proxy.go#L118-L144
		case "no_proxy":
		case "http_proxy":
		case "https_proxy":
		default:
			env = append(env, value)
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
			oci.WithDroppedCapabilities(cap.Known()),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
			oci.WithRootFSPath(filepath.Join(constants.SystemLibexecPath, o.ID(r))),
			oci.WithRootFSReadonly(),
			oci.WithUser(fmt.Sprintf("%d:%d", constants.ApidUserID, constants.ApidUserID)),
		),
		runner.WithOOMScoreAdj(-998),
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
