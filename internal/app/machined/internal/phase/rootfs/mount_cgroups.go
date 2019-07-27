/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/internal/mount/cgroups"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

// MountCgroups represents the MountCgroups task.
type MountCgroups struct{}

// NewMountCgroupsTask initializes and returns an MountCgroups task.
func NewMountCgroupsTask() phase.Task {
	return &MountCgroups{}
}

// RuntimeFunc returns the runtime function.
func (task *MountCgroups) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Standard:
		return task.runtime
	default:
		return nil
	}
}

func (task *MountCgroups) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	if err = cgroups.Mount(); err != nil {
		return errors.Wrap(err, "error mounting cgroups")
	}

	return nil
}
