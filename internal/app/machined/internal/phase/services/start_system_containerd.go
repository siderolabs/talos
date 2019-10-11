/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// StartSystemContainerd represents the task to start system containerd.
type StartSystemContainerd struct{}

// NewStartSystemContainerdTask initializes and returns an Services task.
func NewStartSystemContainerdTask() phase.Task {
	return &StartSystemContainerd{}
}

// TaskFunc returns the runtime function.
func (task *StartSystemContainerd) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *StartSystemContainerd) standard(r runtime.Runtime) (err error) {
	system.Services(r.Config()).LoadAndStart(&services.SystemContainerd{})
	system.Services(r.Config()).LoadAndStart(&services.SystemContainerd{})

	return nil
}
