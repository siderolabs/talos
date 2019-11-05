// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/metadata"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// CheckInstall represents the CheckInstall task.
type CheckInstall struct{}

// NewCheckInstallTask initializes and returns a CheckInstall task.
func NewCheckInstallTask() phase.Task {
	return &CheckInstall{}
}

// TaskFunc returns the runtime function.
func (task *CheckInstall) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *CheckInstall) standard(r runtime.Runtime) (err error) {
	_, err = metadata.Open()
	if err != nil {
		return err
	}

	return err
}
