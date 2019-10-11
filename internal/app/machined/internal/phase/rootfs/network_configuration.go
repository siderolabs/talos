/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs/etc"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// NetworkConfiguration represents the NetworkConfiguration task.
type NetworkConfiguration struct{}

// NewNetworkConfigurationTask initializes and returns an NetworkConfiguration task.
func NewNetworkConfigurationTask() phase.Task {
	return &NetworkConfiguration{}
}

// TaskFunc returns the runtime function.
func (task *NetworkConfiguration) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *NetworkConfiguration) runtime(r runtime.Runtime) (err error) {
	// Create /etc/resolv.conf.
	if err = etc.ResolvConf(); err != nil {
		return err
	}

	return nil
}
