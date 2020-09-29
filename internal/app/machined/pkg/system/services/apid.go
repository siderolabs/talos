// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: golint
package services

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// APID implements the Service interface. It serves as the concrete type with
// the required methods.
type APID struct{}

// ID implements the Service interface.
func (o *APID) ID(r runtime.Runtime) string {
	return "apid"
}

// PreFunc implements the Service interface.
func (o *APID) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return image.Import(ctx, "/usr/images/apid.tar", "talos/apid")
}

// PostFunc implements the Service interface.
func (o *APID) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (o *APID) Condition(r runtime.Runtime) conditions.Condition {
	if r.Config().Machine().Type() == machine.TypeJoin {
		return conditions.WaitForFileToExist(constants.KubeletKubeconfig)
	}

	return nil
}

// DependsOn implements the Service interface.
func (o *APID) DependsOn(r runtime.Runtime) []string {
	if r.State().Platform().Mode() == runtime.ModeContainer || !r.Config().Machine().Time().Enabled() {
		return []string{"containerd", "networkd"}
	}

	return []string{"containerd", "networkd", "timed"}
}

// Runner implements the Service interface.
//
//nolint: gocyclo
func (o *APID) Runner(r runtime.Runtime) (runner.Runner, error) {
	image := "talos/apid"

	endpoints := []string{"127.0.0.1"}

	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.APISocketPath), 0o750); err != nil {
		return nil, err
	}

	if r.Config().Machine().Type() == machine.TypeJoin {
		opts := []retry.Option{retry.WithUnits(3 * time.Second), retry.WithJitter(time.Second)}

		err := retry.Constant(4*time.Minute, opts...).Retry(func() error {
			ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer ctxCancel()

			h, err := kubernetes.NewClientFromKubeletKubeconfig()
			if err != nil {
				return retry.ExpectedError(fmt.Errorf("failed to create client: %w", err))
			}

			endpoints, err = h.MasterIPs(ctx)
			if err != nil {
				return retry.ExpectedError(err)
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Set the process arguments.
	args := runner.Args{
		ID: o.ID(r),
		ProcessArgs: []string{
			"/apid",
			"--endpoints=" + strings.Join(endpoints, ","),
		},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.RouterdSocketPath), Source: filepath.Dir(constants.RouterdSocketPath), Options: []string{"rbind", "ro"}},
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
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
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
