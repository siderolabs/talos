/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"log"
	"os"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

// ExtraEnvVars represents the ExtraEnvVars task.
type ExtraEnvVars struct{}

// NewExtraEnvVarsTask initializes and returns an UserData task.
func NewExtraEnvVarsTask() phase.Task {
	return &ExtraEnvVars{}
}

// RuntimeFunc returns the runtime function.
func (task *ExtraEnvVars) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *ExtraEnvVars) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	// Set the requested environment variables.
	for key, val := range data.Env {
		if err = os.Setenv(key, val); err != nil {
			log.Printf("WARNING failed to set enivronment variable: %v", err)
		}
	}

	return nil
}
