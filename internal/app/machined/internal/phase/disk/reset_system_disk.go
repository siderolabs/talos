// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disk

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice"
)

// ResetSystemDisk represents the task for stop all containerd tasks in the
// k8s.io namespace.
type ResetSystemDisk struct {
	devname string
}

// NewResetSystemDiskTask initializes and returns an Services task.
func NewResetSystemDiskTask(devname string) phase.Task {
	return &ResetSystemDisk{
		devname: devname,
	}
}

// TaskFunc returns the runtime function.
func (task *ResetSystemDisk) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return func(r runtime.Runtime) error {
		return task.standard()
	}
}

func (task *ResetSystemDisk) standard() (err error) {
	return blockdevice.ResetDevice(task.devname)
}
