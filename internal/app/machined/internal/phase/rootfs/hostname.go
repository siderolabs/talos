/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs/etc"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Hostname represents the Hostname task.
type Hostname struct{}

// NewHostnameTask initializes and returns an Hostname task.
func NewHostnameTask() phase.Task {
	return &Hostname{}
}

// RuntimeFunc returns the runtime function.
func (task *Hostname) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *Hostname) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	// Sets the hostname
	return etc.Hosts(data.Networking.OS.Hostname)
}
