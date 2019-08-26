/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"

	"github.com/containerd/containerd/namespaces"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/pkg/userdata"
)

// StopNonCrucialServices represents the task for stop all services to perform
// an upgrade.
type StopNonCrucialServices struct{}

// NewStopNonCrucialServicesTask initializes and returns an Services task.
func NewStopNonCrucialServicesTask() phase.Task {
	return &StopNonCrucialServices{}
}

// RuntimeFunc returns the runtime function.
func (task *StopNonCrucialServices) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return func(platform platform.Platform, data *userdata.UserData) error {
		return task.standard(data)
	}
}

func (task *StopNonCrucialServices) standard(data *userdata.UserData) (err error) {
	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")

	services := []string{"osd", "udevd", "networkd", "ntpd"}
	if data.Services.Kubeadm.IsControlPlane() {
		services = append(services, "trustd", "proxyd")
	}
	for _, service := range services {
		if err = system.Services(nil).Stop(ctx, service); err != nil {
			return err
		}
	}

	return nil
}
