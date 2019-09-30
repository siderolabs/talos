/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"os"
	"path/filepath"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// CheckInstall represents the CheckInstall task.
type CheckInstall struct{}

// NewCheckInstallTask initializes and returns a CheckInstall task.
func NewCheckInstallTask() phase.Task {
	return &CheckInstall{}
}

// RuntimeFunc returns the runtime function.
func (task *CheckInstall) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *CheckInstall) standard(args *phase.RuntimeArgs) (err error) {
	_, err = os.Stat(filepath.Join(constants.BootMountPoint, "installed"))
	return err
}
