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
	"strconv"
	"time"

	"github.com/containerd/containerd/v2/pkg/cap"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/opencontainers/runtime-spec/specs-go"
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
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

var _ system.HealthcheckedService = (*Trustd)(nil)

// Trustd implements the Service interface. It serves as the concrete type with
// the required methods.
type Trustd struct {
	runtimeServer *grpc.Server
}

// ID implements the Service interface.
func (t *Trustd) ID(runtime.Runtime) string {
	return "trustd"
}

// PreFunc implements the Service interface.
//
//nolint:gocyclo
func (t *Trustd) PreFunc(_ context.Context, r runtime.Runtime) error {
	// filter apid access to make sure apid can only access its certificates
	resources := state.Filter(
		r.State().V1Alpha2().Resources(),
		func(ctx context.Context, access state.Access) error {
			if !access.Verb.Readonly() {
				return errors.New("write access denied")
			}

			switch {
			case access.ResourceNamespace == secrets.NamespaceName && access.ResourceType == secrets.TrustdType && access.ResourceID == secrets.TrustdID:
			case access.ResourceNamespace == secrets.NamespaceName && access.ResourceType == secrets.OSRootType && access.ResourceID == secrets.OSRootID:
			default:
				return errors.New("access denied")
			}

			return nil
		},
	)

	// ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.TrustdRuntimeSocketPath), 0o750); err != nil {
		return err
	}

	// set the final leaf to be world-executable to make trustd connect to the socket
	if err := os.Chmod(filepath.Dir(constants.TrustdRuntimeSocketPath), 0o751); err != nil {
		return err
	}

	// clean up the socket if it already exists (important for Talos in a container)
	if err := os.RemoveAll(constants.TrustdRuntimeSocketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", constants.TrustdRuntimeSocketPath)
	if err != nil {
		return err
	}

	if err := selinux.SetLabel(constants.TrustdRuntimeSocketPath, constants.TrustdRuntimeSocketLabel); err != nil {
		return err
	}

	// chown the socket path to make it accessible to the apid
	if err := os.Chown(constants.TrustdRuntimeSocketPath, constants.TrustdUserID, constants.TrustdUserID); err != nil {
		return err
	}

	t.runtimeServer = grpc.NewServer(
		grpc.SharedWriteBuffer(true),
	)
	v1alpha1.RegisterStateServer(t.runtimeServer, server.NewState(resources))

	go t.runtimeServer.Serve(listener) //nolint:errcheck

	return prepareRootfs(t.ID(r))
}

// PostFunc implements the Service interface.
func (t *Trustd) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
	t.runtimeServer.Stop()

	return os.RemoveAll(constants.TrustdRuntimeSocketPath)
}

// Condition implements the Service interface.
func (t *Trustd) Condition(r runtime.Runtime) conditions.Condition {
	return conditions.WaitForAll(
		timeresource.NewSyncCondition(r.State().V1Alpha2().Resources()),
		network.NewReadyCondition(r.State().V1Alpha2().Resources(), network.AddressReady, network.HostnameReady),
	)
}

// DependsOn implements the Service interface.
func (t *Trustd) DependsOn(runtime.Runtime) []string {
	return []string{"containerd"}
}

// Volumes implements the Service interface.
func (t *Trustd) Volumes() []string {
	return nil
}

// Runner implements the Service interface.
func (t *Trustd) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Set the process arguments.
	args := runner.Args{
		ID:          t.ID(r),
		ProcessArgs: []string{"/trustd"},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: filepath.Dir(constants.TrustdRuntimeSocketPath), Source: filepath.Dir(constants.TrustdRuntimeSocketPath), Options: []string{"rbind", "ro"}},
	}

	env := environment.Get(r.Config())
	env = append(env,
		constants.TcellMinimizeEnvironment,
		"GOMEMLIMIT="+strconv.Itoa(constants.CgroupTrustdMaxMemory/5*4),
	)

	if debug.RaceEnabled {
		env = append(env, "GORACE=halt_on_error=1")
	}

	return restart.New(containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithLoggingManager(r.Logging()),
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithEnv(env),
		runner.WithCgroupPath(constants.CgroupTrustd),
		runner.WithGracefulShutdownTimeout(15*time.Second),
		runner.WithSelinuxLabel(constants.SelinuxLabelTrustd),
		runner.WithOCISpecOpts(
			oci.WithDroppedCapabilities(cap.Known()),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
			oci.WithRootFSPath(filepath.Join(constants.SystemLibexecPath, t.ID(r))),
			oci.WithRootFSReadonly(),
			oci.WithUser(fmt.Sprintf("%d:%d", constants.TrustdUserID, constants.TrustdUserID)),
		),
		runner.WithOOMScoreAdj(-998),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (t *Trustd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer

		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", "127.0.0.1", constants.TrustdPort))
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (t *Trustd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
