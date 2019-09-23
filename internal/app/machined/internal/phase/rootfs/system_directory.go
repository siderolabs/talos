/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"os"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

// SystemDirectory represents the SystemDirectory task.
type SystemDirectory struct{}

// NewSystemDirectoryTask initializes and returns an SystemDirectory task.
func NewSystemDirectoryTask() phase.Task {
	return &SystemDirectory{}
}

// RuntimeFunc returns the runtime function.
func (task *SystemDirectory) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *SystemDirectory) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	return os.MkdirAll("/run/system/etc", os.ModeDir)
}
