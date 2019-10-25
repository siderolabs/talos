// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/virtual"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// MountSubDevices represents the MountSubDevices task.
type MountSubDevices struct{}

// NewMountSubDevicesTask initializes and returns an MountSubDevices task.
func NewMountSubDevicesTask() phase.Task {
	return &MountSubDevices{}
}

// TaskFunc returns the runtime function.
func (task *MountSubDevices) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *MountSubDevices) runtime(r runtime.Runtime) (err error) {
	var mountpoints *mount.Points

	mountpoints, err = virtual.SubMountPoints()
	if err != nil {
		return err
	}

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
