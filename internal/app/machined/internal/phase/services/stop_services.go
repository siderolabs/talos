/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// StopServices represents the StopServices task.
type StopServices struct{}

// NewStopServicesTask initializes and returns an Services task.
func NewStopServicesTask() phase.Task {
	return &StopServices{}
}

// TaskFunc returns the runtime function.
func (task *StopServices) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return func(r runtime.Runtime) error {
		return task.standard()
	}
}

func (task *StopServices) standard() (err error) {
	system.Services(nil).Shutdown()
	return nil
}
