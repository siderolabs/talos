// +build linux,arm64 linux,arm

/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package vmware

import (
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/pkg/userdata"
)

// VMware is the concrete type that implements the platform.Platform interface.
type VMware struct{}

// Name implements the platform.Platform interface.
func (vmw *VMware) Name() string {
	return "VMware"
}

// UserData implements the platform.Platform interface.
func (vmw *VMware) UserData() (data *userdata.UserData, err error) {
	return nil, errors.New("not implemented")
}

// Prepare implements the platform.Platform interface and handles initial host preparation.
func (vmw *VMware) Prepare(data *userdata.UserData) (err error) {
	return errors.New("not implemented")
}

// Install implements the platform.Platform interface and handles additional system setup.
func (vmw *VMware) Install(data *userdata.UserData) (err error) {
	return errors.New("not implemented")
}
