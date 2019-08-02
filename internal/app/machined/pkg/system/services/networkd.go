/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Networkd implements the Service interface. It serves as the concrete type with
// the required methods.
type Networkd struct{}

// ID implements the Service interface.
func (n *Networkd) ID(data *userdata.UserData) string {
	return "networkd"
}

// PreFunc implements the Service interface.
func (n *Networkd) PreFunc(ctx context.Context, data *userdata.UserData) error {
	return containerd.Import(constants.SystemContainerdNamespace, &containerd.ImportRequest{
		Path: "/usr/images/networkd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/networkd"),
		},
	})
}

// PostFunc implements the Service interface.
func (n *Networkd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (n *Networkd) Condition(data *userdata.UserData) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (n *Networkd) DependsOn(data *userdata.UserData) []string {
	return []string{"containerd"}
}

func (n *Networkd) Runner(data *userdata.UserData) (runner.Runner, error) {
	image := "talos/networkd"

	args := runner.Args{
		ID:          n.ID(data),
		ProcessArgs: []string{"/networkd"},
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/resolv.conf", Source: "/etc/resolv.conf", Options: []string{"rbind", "rw"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		data,
		&args,
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}
