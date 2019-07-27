/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package sysctls

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/pkg/proc"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Sysctls represents the Sysctls task.
type Sysctls struct{}

// NewSysctlsTask initializes and returns an UserData task.
func NewSysctlsTask() phase.Task {
	return &Sysctls{}
}

// RuntimeFunc returns the runtime function.
func (task *Sysctls) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *Sysctls) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	return proc.WriteSystemProperty(&proc.SystemProperty{Key: "net.ipv4.ip_forward", Value: "1"})
}
