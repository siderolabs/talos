// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// ValidateConfig represents the ValidateConfig task.
type ValidateConfig struct{}

// NewValidateConfigTask initializes and returns a ValidateConfig task.
func NewValidateConfigTask() phase.Task {
	return &ValidateConfig{}
}

// TaskFunc returns the runtime function.
func (task *ValidateConfig) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *ValidateConfig) standard(r runtime.Runtime) (err error) {
	return r.Config().Validate(r.Platform().Mode())
}
