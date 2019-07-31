/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"fmt"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/pkg/userdata"
	"golang.org/x/sys/unix"
)

// ExtraDevices represents the ExtraDevices task.
type ExtraDevices struct{}

// NewExtraDevicesTask initializes and returns an UserData task.
func NewExtraDevicesTask() phase.Task {
	return &ExtraDevices{}
}

// RuntimeFunc returns the runtime function.
func (task *ExtraDevices) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *ExtraDevices) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	if data.Install == nil || data.Install.ExtraDevices == nil {
		return nil
	}

	mountpoints := mount.NewMountPoints()
	for _, extra := range data.Install.ExtraDevices {
		for i, part := range extra.Partitions {
			devname := fmt.Sprintf("%s%d", extra.Device, i+1)
			mountpoints.Set(devname, mount.NewMountPoint(devname, part.MountPoint, "xfs", unix.MS_NOATIME, ""))
		}
	}

	extras := manager.NewManager(mountpoints)
	if err = extras.MountAll(); err != nil {
		return err
	}

	return nil
}
