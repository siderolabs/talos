// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: golint
package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/oci"
	"github.com/golang/protobuf/ptypes/empty"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/syndtr/gocapability/capability"
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
	healthapi "github.com/talos-systems/talos/pkg/machinery/api/health"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Timed implements the Service interface. It serves as the concrete type with
// the required methods.
type Timed struct{}

// ID implements the Service interface.
func (n *Timed) ID(r runtime.Runtime) string {
	return "timed"
}

// PreFunc implements the Service interface.
func (n *Timed) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return image.Import(ctx, "/usr/images/timed.tar", "talos/timed")
}

// PostFunc implements the Service interface.
func (n *Timed) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (n *Timed) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (n *Timed) DependsOn(r runtime.Runtime) []string {
	return []string{"containerd", "networkd"}
}

func (n *Timed) Runner(r runtime.Runtime) (runner.Runner, error) {
	image := "talos/timed"

	args := runner.Args{
		ID:          n.ID(r),
		ProcessArgs: []string{"/timed"},
	}

	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.TimeSocketPath), 0o750); err != nil {
		return nil, err
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: filepath.Dir(constants.TimeSocketPath), Source: filepath.Dir(constants.TimeSocketPath), Options: []string{"rbind", "rw"}},
	}

	env := []string{}
	for key, val := range r.Config().Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	b, err := r.Config().Bytes()
	if err != nil {
		return nil, err
	}

	stdin := bytes.NewReader(b)

	return restart.New(containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithStdin(stdin),
		runner.WithLoggingManager(r.Logging()),
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*32)),
			oci.WithCapabilities([]string{
				strings.ToUpper("CAP_" + capability.CAP_SYS_TIME.String()),
			}),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// APIStartAllowed implements the APIStartableService interface.
func (n *Timed) APIStartAllowed(r runtime.Runtime) bool {
	return true
}

// APIRestartAllowed implements the APIRestartableService interface.
func (n *Timed) APIRestartAllowed(r runtime.Runtime) bool {
	return true
}

// HealthFunc implements the HealthcheckedService interface.
func (n *Timed) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		var (
			conn      *grpc.ClientConn
			err       error
			hcResp    *healthapi.HealthCheckResponse
			readyResp *healthapi.ReadyCheckResponse
		)

		conn, err = grpc.DialContext(
			ctx,
			fmt.Sprintf("%s://%s", "unix", constants.TimeSocketPath),
			grpc.WithInsecure(),
			grpc.WithContextDialer(dialer.DialUnix()),
		)
		if err != nil {
			return err
		}
		defer conn.Close() //nolint: errcheck

		nClient := healthapi.NewHealthClient(conn)
		if readyResp, err = nClient.Ready(ctx, &empty.Empty{}); err != nil {
			return err
		}

		if readyResp.Messages[0].Status != healthapi.ReadyCheck_READY {
			return errors.New("timed is not ready")
		}

		if hcResp, err = nClient.Check(ctx, &empty.Empty{}); err != nil {
			return err
		}

		if hcResp.Messages[0].Status == healthapi.HealthCheck_SERVING {
			return nil
		}

		return fmt.Errorf("timed is unhealthy: %s", hcResp.Messages[0].Status.String())
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (n *Timed) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
