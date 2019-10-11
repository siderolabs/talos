/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/overlay"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// MountOverlay represents the MountOverlay task.
type MountOverlay struct{}

// NewMountOverlayTask initializes and returns an MountOverlay task.
func NewMountOverlayTask() phase.Task {
	return &MountOverlay{}
}

// TaskFunc returns the runtime function.
func (task *MountOverlay) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *MountOverlay) standard(r runtime.Runtime) (err error) {
	var mountpoints *mount.Points

	mountpoints, err = overlay.MountPoints()
	if err != nil {
		return err
	}

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
