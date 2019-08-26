/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/overlay"
	"github.com/talos-systems/talos/pkg/userdata"
)

// UnmountOverlay represents the UnmountOverlay task.
type UnmountOverlay struct{}

// NewUnmountOverlayTask initializes and returns an UnmountOverlay task.
func NewUnmountOverlayTask() phase.Task {
	return &UnmountOverlay{}
}

// RuntimeFunc returns the runtime function.
func (task *UnmountOverlay) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *UnmountOverlay) standard(platform platform.Platform, data *userdata.UserData) (err error) {
	var mountpoints *mount.Points
	mountpoints, err = overlay.MountPoints()
	if err != nil {
		return err
	}

	m := manager.NewManager(mountpoints)
	if err = m.UnmountAll(); err != nil {
		return err
	}

	return nil
}
