/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"fmt"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// NTPd implements the Service interface. It serves as the concrete type with
// the required methods.
type NTPd struct{}

// ID implements the Service interface.
func (n *NTPd) ID(data *userdata.UserData) string {
	return "ntpd"
}

// PreFunc implements the Service interface.
func (n *NTPd) PreFunc(data *userdata.UserData) error {
	return nil
}

// PostFunc implements the Service interface.
func (n *NTPd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (n *NTPd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

func (n *NTPd) Start(data *userdata.UserData) error {
	image := "talos/ntpd"

	args := runner.Args{
		ID:          n.ID(data),
		ProcessArgs: []string{"/ntpd", "--userdata=" + constants.UserDataPath},
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
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
			containerd.WithMemoryLimit(int64(1000000*32)),
			oci.WithMounts(mounts),
		),
	)
}
