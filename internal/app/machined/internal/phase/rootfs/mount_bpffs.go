/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/bpffs"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// MountBPFFS represents the MountBPFFS task.
type MountBPFFS struct{}

// NewMountBPFFSTask initializes and returns an MountBPFFS task.
func NewMountBPFFSTask() phase.Task {
	return &MountBPFFS{}
}

// RuntimeFunc returns the runtime function.
func (task *MountBPFFS) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *MountBPFFS) runtime(args *phase.RuntimeArgs) (err error) {
	var mountpoints *mount.Points
	mountpoints, err = bpffs.MountPoints()
	if err != nil {
		return err
	}

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
