// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"fmt"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// UnmountSystemDisks represents the UnmountSystemDisks task.
type UnmountSystemDisks struct {
	devlabel string
}

// NewUnmountSystemDisksTask initializes and returns an UnmountSystemDisks task.
func NewUnmountSystemDisksTask(devlabel string) phase.Task {
	return &UnmountSystemDisks{
		devlabel: devlabel,
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
	mountpoints := mount.NewMountPoints()

	mountpoint, err := owned.MountPointForLabel(task.devlabel)
	if err != nil {
		return err
	}

	mountpoints.Set(task.devlabel, mountpoint)

	unix.Sync()

	m := manager.NewManager(mountpoints)
	if err = m.UnmountAll(); err != nil {
		return fmt.Errorf("error unmounting %q partition: %w", task.devlabel, err)
	}

	return nil
}
