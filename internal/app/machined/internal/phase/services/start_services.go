/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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

// RuntimeFunc returns the runtime function.
func (task *StartServices) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.standard
}

func (task *StartServices) standard(args *phase.RuntimeArgs) (err error) {
	task.loadSystemServices(args)
	task.loadKubernetesServices(args)

	system.Services(args.Config()).StartAll()

	return nil
}

func (task *StartServices) loadSystemServices(args *phase.RuntimeArgs) {
	svcs := system.Services(args.Config())
	// Start the services common to all nodes.
	svcs.Load(
		&services.MachinedAPI{},
		&services.Containerd{},
		&services.OSD{},
		&services.Networkd{},
	)

	if args.Platform().Mode() != runtime.Container {
		svcs.Load(
			&services.NTPd{},
			&services.Udevd{},
			&services.UdevdTrigger{},
		)
	}

	// Start the services common to all control plane nodes.

	switch args.Config().Machine().Type() {
	case machine.Bootstrap:
		fallthrough
	case machine.ControlPlane:
		svcs.Load(
			&services.Etcd{},
			&services.Trustd{},
		)
	}
}

func (task *StartServices) loadKubernetesServices(args *phase.RuntimeArgs) {
	svcs := system.Services(args.Config())
	svcs.Load(
		&services.Kubelet{},
	)

	if args.Config().Machine().Type() == machine.Bootstrap {
		svcs.Load(
			&services.Bootkube{},
		)
	}
}
