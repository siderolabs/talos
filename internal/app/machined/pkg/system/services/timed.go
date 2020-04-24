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

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/pkg/constants"
)

// Timed implements the Service interface. It serves as the concrete type with
// the required methods.
type Timed struct{}

// ID implements the Service interface.
func (n *Timed) ID(config runtime.Configurator) string {
	return "timed"
}

// PreFunc implements the Service interface.
func (n *Timed) PreFunc(ctx context.Context, config runtime.Configurator) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))

	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/timed.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/timed"),
		},
	})
}

// PostFunc implements the Service interface.
func (n *Timed) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (n *Timed) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (n *Timed) DependsOn(config runtime.Configurator) []string {
	return []string{"containerd", "networkd"}
}

func (n *Timed) Runner(config runtime.Configurator) (runner.Runner, error) {
	image := "talos/timed"

	args := runner.Args{
		ID:          n.ID(config),
		ProcessArgs: []string{"/timed", "--config=" + constants.ConfigPath},
	}

	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.TimeSocketPath), 0750); err != nil {
		return nil, err
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.TimeSocketPath), Source: filepath.Dir(constants.TimeSocketPath), Options: []string{"rbind", "rw"}},
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
func (n *Timed) APIStartAllowed(config runtime.Configurator) bool {
	return true
}

// APIRestartAllowed implements the APIRestartableService interface.
func (n *Timed) APIRestartAllowed(config runtime.Configurator) bool {
	return true
}
