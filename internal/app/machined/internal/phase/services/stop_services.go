// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
)

// StopServices represents the StopServices task.
type StopServices struct {
	upgrade bool
}

// NewStopServicesTask initializes and returns an Services task.
func NewStopServicesTask(upgrade bool) phase.Task {
	return &StopServices{
		upgrade: upgrade,
	}
}

// TaskFunc returns the runtime function.
func (task *StopServices) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *StopServices) standard(r runtime.Runtime) (err error) {
	if task.upgrade {
		services := []string{"containerd", "networkd", "ntpd", "udevd"}

		if r.Config().Machine().Type() == machine.TypeBootstrap || r.Config().Machine().Type() == machine.TypeControlPlane {
			services = append(services, "trustd")
		}

		for _, service := range services {
			if err = system.Services(nil).Stop(context.Background(), service); err != nil {
				return err
			}
		}

		return nil
	}

	system.Services(nil).Shutdown()

	return nil
}
