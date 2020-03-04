// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	"github.com/golang/protobuf/ptypes/empty"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/syndtr/gocapability/capability"
	"google.golang.org/grpc"

	healthapi "github.com/talos-systems/talos/api/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/dialer"
)

// Networkd implements the Service interface. It serves as the concrete type with
// the required methods.
type Networkd struct{}

// ID implements the Service interface.
func (n *Networkd) ID(config runtime.Configurator) string {
	return "networkd"
}

// PreFunc implements the Service interface.
func (n *Networkd) PreFunc(ctx context.Context, config runtime.Configurator) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))

	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/networkd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/networkd"),
		},
	})
}

// PostFunc implements the Service interface.
func (n *Networkd) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (n *Networkd) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (n *Networkd) DependsOn(config runtime.Configurator) []string {
	return []string{"system-containerd"}
}

func (n *Networkd) Runner(config runtime.Configurator) (runner.Runner, error) {
	image := "talos/networkd"

	args := runner.Args{
		ID: n.ID(config),
		ProcessArgs: []string{
			"/networkd",
			"--config=" + constants.ConfigPath,
		},
	}

	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.NetworkSocketPath), 0750); err != nil {
		return nil, err
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/resolv.conf", Source: "/etc/resolv.conf", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: "/etc/hosts", Source: "/etc/hosts", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: filepath.Dir(constants.NetworkSocketPath), Source: filepath.Dir(constants.NetworkSocketPath), Options: []string{"rbind", "rw"}},
	}

	env := []string{}
	for key, val := range config.Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	// This is really only here to support container runtime
	if p, ok := os.LookupEnv("PLATFORM"); ok {
		env = append(env, fmt.Sprintf("%s=%s", "PLATFORM", p))
	}

	return restart.New(containerd.NewRunner(
		config.Debug(),
		&args,
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*32)),
			oci.WithCapabilities([]string{
				strings.ToUpper("CAP_" + capability.CAP_NET_ADMIN.String()),
				strings.ToUpper("CAP_" + capability.CAP_SYS_ADMIN.String()),
				strings.ToUpper("CAP_" + capability.CAP_NET_RAW.String()),
			}),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (n *Networkd) HealthFunc(runtime.Configurator) health.Check {
	return func(ctx context.Context) error {
		var (
			conn      *grpc.ClientConn
			err       error
			hcResp    *healthapi.HealthCheckResponse
			readyResp *healthapi.ReadyCheckResponse
		)

		conn, err = grpc.Dial(
			fmt.Sprintf("%s://%s", "unix", constants.NetworkSocketPath),
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
			return errors.New("networkd is not ready")
		}

		if hcResp, err = nClient.Check(ctx, &empty.Empty{}); err != nil {
			return err
		}

		if hcResp.Messages[0].Status == healthapi.HealthCheck_SERVING {
			return nil
		}

		return errors.New("networkd is unhealthy")
	}
}

// HealthSettings implements the HealthcheckedService interface
func (n *Networkd) HealthSettings(runtime.Configurator) *health.Settings {
	return &health.DefaultSettings
}
