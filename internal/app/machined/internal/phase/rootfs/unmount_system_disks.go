/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// UnmountSystemDisks represents the UnmountSystemDisks task.
type UnmountSystemDisks struct {
	devname string
}

// NewUnmountSystemDisksTask initializes and returns an UnmountSystemDisks task.
func NewUnmountSystemDisksTask(devname string) phase.Task {
	return &UnmountSystemDisks{
		devname: devname,
	}
}

// TaskFunc returns the runtime function.
func (task *UnmountSystemDisks) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *UnmountSystemDisks) standard(r runtime.Runtime) (err error) {
	var mountpoints *mount.Points

	mountpoints, err = owned.MountPointsForDevice(task.devname)
	if err != nil {
		return err
	}

	m := manager.NewManager(mountpoints)
	if err = m.UnmountAll(); err != nil {
		return err
	}

	return nil
}
