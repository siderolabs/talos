// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/sysctl"
)

// Sysctls represents the Sysctls task.
type Sysctls struct{}

// NewSysctlsTask initializes and returns a Sysctls task.
func NewSysctlsTask() phase.Task {
	return &Sysctls{}
}

// TaskFunc returns the runtime function.
func (task *Sysctls) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.runtime
}

func (task *Sysctls) runtime(r runtime.Runtime) (err error) {
	var result *multierror.Error

	for k, v := range r.Config().Machine().Sysctls() {
		if err = sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: k, Value: v}); err != nil {
			return err
		}
	}

	return result.ErrorOrNil()
}
