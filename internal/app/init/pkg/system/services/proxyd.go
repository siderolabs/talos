/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"fmt"
	"os"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Proxyd implements the Service interface. It serves as the concrete type with
// the required methods.
type Proxyd struct{}

// ID implements the Service interface.
func (p *Proxyd) ID(data *userdata.UserData) string {
	return "proxyd"
}

// PreFunc implements the Service interface.
func (p *Proxyd) PreFunc(data *userdata.UserData) error {
	return os.MkdirAll("/etc/kubernetes", os.ModeDir)
}

// PostFunc implements the Service interface.
func (p *Proxyd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (p *Proxyd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.WaitForFilesToExist("/etc/kubernetes/pki/ca.crt", "/etc/kubernetes/admin.conf")
}

func (p *Proxyd) Start(data *userdata.UserData) error {
	image := "talos/proxyd"

	// Set the process arguments.
	args := runner.Args{
		ID:          p.ID(data),
		ProcessArgs: []string{"/proxyd"},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes/admin.conf", Source: "/etc/kubernetes/admin.conf", Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes/pki/ca.crt", Source: "/etc/kubernetes/pki/ca.crt", Options: []string{"rbind", "ro"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*512)),
			oci.WithMounts(mounts),
			oci.WithPrivileged,
		),
	)
}
