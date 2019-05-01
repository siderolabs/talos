/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"fmt"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Udevd implements the Service interface. It serves as the concrete type with
// the required methods.
type Udevd struct{}

// ID implements the Service interface.
func (c *Udevd) ID(data *userdata.UserData) string {
	return "udevd"
}

// PreFunc implements the Service interface.
func (c *Udevd) PreFunc(data *userdata.UserData) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *Udevd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (c *Udevd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

// Runner implements the Service interface.
func (c *Udevd) Runner(data *userdata.UserData) (runner.Runner, error) {
	image := "talos/udevd"

	// Set the process arguments.
	args := runner.Args{
		ID:          c.ID(data),
		ProcessArgs: []string{"/udevd"},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		// NB: We must mount /dev to ensure that changes on the host are reflected in the container.
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
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
			containerd.WithRootfsPropagation("shared"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithHostNamespace(specs.UTSNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
	),
		restart.WithType(restart.Forever),
	), nil
}
