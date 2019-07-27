/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/mount"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
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
	// Mount the extra devices.
	if err = mount.ExtraDevices(data); err != nil {
		return err
	}

	return nil
}
