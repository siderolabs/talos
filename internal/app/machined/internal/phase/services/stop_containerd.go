// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// StopContainerd represents the task for stop all services to perform
// an upgrade.
type StopContainerd struct{}

// NewStopContainerdTask initializes and returns an Services task.
func NewStopContainerdTask() phase.Task {
	return &StopContainerd{}
}

// TaskFunc returns the runtime function.
func (task *StopContainerd) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return func(r runtime.Runtime) error {
		return task.standard()
	}
}

func (task *StopContainerd) standard() (err error) {
	if err = system.Services(nil).Stop(context.Background(), "containerd"); err != nil {
		return err
	}

	return nil
}
