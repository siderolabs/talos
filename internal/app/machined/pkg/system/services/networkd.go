/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

// Networkd implements the Service interface. It serves as the concrete type with
// the required methods.
type Networkd struct{}

// ID implements the Service interface.
func (n *Networkd) ID(config config.Configurator) string {
	return "networkd"
}

// PreFunc implements the Service interface.
func (n *Networkd) PreFunc(ctx context.Context, config config.Configurator) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))
	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/networkd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/networkd"),
		},
	})
}

// PostFunc implements the Service interface.
func (n *Networkd) PostFunc(config config.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (n *Networkd) Condition(config config.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (n *Networkd) DependsOn(config config.Configurator) []string {
	return []string{"system-containerd"}
}

func (n *Networkd) Runner(config config.Configurator) (runner.Runner, error) {
	image := "talos/networkd"

	args := runner.Args{
		ID: n.ID(config),
		ProcessArgs: []string{
			"/networkd",
			"--config=" + constants.ConfigPath,
		},
	}

	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.NetworkdSocketPath), os.ModeDir); err != nil {
		return nil, err
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/resolv.conf", Source: "/etc/resolv.conf", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: "/etc/hosts", Source: "/etc/hosts", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: filepath.Dir(constants.NetworkdSocketPath), Source: filepath.Dir(constants.NetworkdSocketPath), Options: []string{"rbind", "rw"}},
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
