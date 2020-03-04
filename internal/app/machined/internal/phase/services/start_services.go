// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
)

// StartServices represents the StartServices task.
type StartServices struct{}

// NewStartServicesTask initializes and returns an Services task.
func NewStartServicesTask() phase.Task {
	return &StartServices{}
}

// TaskFunc returns the runtime function.
func (task *StartServices) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *StartServices) standard(r runtime.Runtime) (err error) {
	task.loadSystemServices(r)
	task.loadKubernetesServices(r)

	system.Services(r.Config()).StartAll()

	return nil
}

func (task *StartServices) loadSystemServices(r runtime.Runtime) {
	svcs := system.Services(r.Config())
	// Start the services common to all nodes.
	svcs.Load(
		&services.MachinedAPI{},
		&services.Containerd{},
		&services.APID{},
		&services.OSD{},
		&services.Networkd{},
		&services.Routerd{},
	)

	if r.Platform().Mode() != runtime.Container {
		// udevd-trigger is causing stalls/unresponsive stuff when running in local mode
		// TODO: investigate root cause, but workaround for now is to skip it in container mode
		svcs.Load(
			&services.NTPd{},
			&services.Udevd{},
			&services.UdevdTrigger{},
		)
	}

	// Start the services common to all control plane nodes.

	switch r.Config().Machine().Type() {
	case machine.TypeInit:
		fallthrough
	case machine.TypeControlPlane:
		svcs.Load(
			&services.Etcd{},
			&services.Trustd{},
		)
	}
}

func (task *StartServices) loadKubernetesServices(r runtime.Runtime) {
	svcs := system.Services(r.Config())
	svcs.Load(
		&services.Kubelet{},
	)

	if r.Config().Machine().Type() == machine.TypeInit {
		svcs.Load(
			&services.Bootkube{},
		)
	}
}
