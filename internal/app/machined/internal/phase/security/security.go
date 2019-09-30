/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package security

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/kernel/kspp"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// Security represents the Security task.
type Security struct{}

// NewSecurityTask initializes and returns an Security task.
func NewSecurityTask() phase.Task {
	return &Security{}
}

// RuntimeFunc returns the runtime function.
func (task *Security) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *Security) runtime(args *phase.RuntimeArgs) (err error) {
	if err = kspp.EnforceKSPPKernelParameters(); err != nil {
		return err
	}
	if err = kspp.EnforceKSPPSysctls(); err != nil {
		return err
	}

	return nil
}
