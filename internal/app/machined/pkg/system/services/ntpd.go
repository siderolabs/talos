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

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// NTPd implements the Service interface. It serves as the concrete type with
// the required methods.
type NTPd struct{}

// ID implements the Service interface.
func (n *NTPd) ID(config runtime.Configurator) string {
	return "ntpd"
}

// PreFunc implements the Service interface.
func (n *NTPd) PreFunc(ctx context.Context, config runtime.Configurator) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))

	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/ntpd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/ntpd"),
		},
	})
}

// PostFunc implements the Service interface.
func (n *NTPd) PostFunc(config runtime.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (n *NTPd) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (n *NTPd) DependsOn(config runtime.Configurator) []string {
	return []string{"system-containerd", "networkd"}
}

func (n *NTPd) Runner(config runtime.Configurator) (runner.Runner, error) {
	image := "talos/ntpd"

	args := runner.Args{
		ID:          n.ID(config),
		ProcessArgs: []string{"/ntpd", "--config=" + constants.ConfigPath},
	}

	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.NtpdSocketPath), os.ModeDir); err != nil {
		return nil, err
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.NtpdSocketPath), Source: filepath.Dir(constants.NtpdSocketPath), Options: []string{"rbind", "rw"}},
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
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// APIStartAllowed implements the APIStartableService interface.
func (n *NTPd) APIStartAllowed(config runtime.Configurator) bool {
	return true
}

// APIRestartAllowed implements the APIRestartableService interface.
func (n *NTPd) APIRestartAllowed(config runtime.Configurator) bool {
	return true
}
