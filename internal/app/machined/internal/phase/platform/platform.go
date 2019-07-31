/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package platform

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Platform represents the Platform task.
type Platform struct{}

// NewPlatformTask initializes and returns an Platform task.
func NewPlatformTask() phase.Task {
	return &Platform{}
}

// RuntimeFunc returns the runtime function.
func (task *Platform) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *Platform) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	if err = platform.Initialize(data); err != nil {
		return err
	}

	return nil
}
