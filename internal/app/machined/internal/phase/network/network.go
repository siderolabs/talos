/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs/etc"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/network"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// UserDefinedNetwork represents the UserDefinedNetwork task.
type UserDefinedNetwork struct{}

// NewUserDefinedNetworkTask initializes and returns an UserDefinedNetwork task.
func NewUserDefinedNetworkTask() phase.Task {
	return &UserDefinedNetwork{}
}

// RuntimeFunc returns the runtime function.
func (task *UserDefinedNetwork) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Standard:
		return task.runtime
	default:
		return nil
	}
}

func (task *UserDefinedNetwork) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	if err = network.SetupNetwork(data); err != nil {
		return err
	}

	// Create /etc/hosts.
	var hostname string
	kernelHostname := kernel.ProcCmdline().Get(constants.KernelParamHostname).First()
	switch {
	case data.Networking != nil && data.Networking.OS != nil && data.Networking.OS.Hostname != "":
		hostname = data.Networking.OS.Hostname
	case kernelHostname != nil:
		hostname = *kernelHostname
	default:
		// will default in Hosts()
	}

	if err = etc.Hosts(hostname); err != nil {
		return err
	}
	return nil
}
