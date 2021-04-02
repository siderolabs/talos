// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint
package services

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path/filepath"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	timeresource "github.com/talos-systems/talos/pkg/resources/time"
)

// Trustd implements the Service interface. It serves as the concrete type with
// the required methods.
type Trustd struct{}

// ID implements the Service interface.
func (t *Trustd) ID(r runtime.Runtime) string {
	return "trustd"
}

// PreFunc implements the Service interface.
func (t *Trustd) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return prepareRootfs(t.ID(r))
}

// PostFunc implements the Service interface.
func (t *Trustd) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (t *Trustd) Condition(r runtime.Runtime) conditions.Condition {
	return timeresource.NewSyncCondition(r.State().V1Alpha2().Resources())
}

// DependsOn implements the Service interface.
func (t *Trustd) DependsOn(r runtime.Runtime) []string {
	return []string{"containerd", "networkd"}
}

func (t *Trustd) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Set the process arguments.
	args := runner.Args{
		ID:          t.ID(r),
		ProcessArgs: []string{"/trustd"},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
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
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*512)),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
			oci.WithRootFSPath(filepath.Join(constants.SystemLibexecPath, t.ID(r))),
			oci.WithRootFSReadonly(),
		),
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
