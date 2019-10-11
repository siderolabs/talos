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

// SystemDirectory represents the SystemDirectory task.
type SystemDirectory struct{}

// NewSystemDirectoryTask initializes and returns an SystemDirectory task.
func NewSystemDirectoryTask() phase.Task {
	return &SystemDirectory{}
}

// TaskFunc returns the runtime function.
func (task *SystemDirectory) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.runtime
}

func (task *SystemDirectory) runtime(r runtime.Runtime) (err error) {
	for _, p := range []string{"etc", "log"} {
		if err = os.MkdirAll(filepath.Join(constants.SystemRunPath, p), 0700); err != nil {
			return err
		}
	}

	return nil
}
