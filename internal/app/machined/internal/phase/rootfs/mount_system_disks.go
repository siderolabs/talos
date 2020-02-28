// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"log"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// MountSystemDisks represents the MountSystemDisks task.
type MountSystemDisks struct {
	devlabel string
	opts     []mount.Option
}

// NewMountSystemDisksTask initializes and returns an MountSystemDisks task.
func NewMountSystemDisksTask(devlabel string, opts ...mount.Option) phase.Task {
	return &MountSystemDisks{
		devlabel: devlabel,
		opts:     opts,
	}
}

// TaskFunc returns the runtime function.
func (task *MountSystemDisks) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *MountSystemDisks) runtime(r runtime.Runtime) (err error) {
	mountpoints := mount.NewMountPoints()

	log.Printf("fetching mountpoint for label %q\n", task.devlabel)

	mountpoint, err := owned.MountPointForLabel(task.devlabel, task.opts...)
	if err != nil {
		return err
	}

	if mountpoint == nil {
		log.Printf("could not find boot partition with label %q\n", task.devlabel)
		return nil
	}

	mountpoints.Set(task.devlabel, mountpoint)

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
