/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

// StartSystemContainerd represents the task to start system containerd.
type StartSystemContainerd struct{}

// NewStartSystemContainerdTask initializes and returns an Services task.
func NewStartSystemContainerdTask() phase.Task {
	return &StartSystemContainerd{}
}

// RuntimeFunc returns the runtime function.
func (task *StartSystemContainerd) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return func(platform platform.Platform, data *userdata.UserData) error {
		return task.standard(data)
	}
}

func (task *StartSystemContainerd) standard(data *userdata.UserData) (err error) {
	system.Services(data).LoadAndStart(&services.SystemContainerd{})

	return nil
}
