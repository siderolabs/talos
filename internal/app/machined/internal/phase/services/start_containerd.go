// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"time"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// StartContainerd represents the task to start system containerd.
type StartContainerd struct{}

// NewStartContainerdTask initializes and returns an Services task.
func NewStartContainerdTask() phase.Task {
	return &StartContainerd{}
}

// TaskFunc returns the runtime function.
func (task *StartContainerd) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *StartContainerd) standard(r runtime.Runtime) (err error) {
	svc := &services.Containerd{}

	system.Services(r.Config()).LoadAndStart(svc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return system.WaitForService(system.StateEventUp, svc.ID(r.Config())).Wait(ctx)
}
