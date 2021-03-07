// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

// Controller interface should be implemented by the VM to be controlled via the API.
type Controller interface {
	PowerOn() error
	PowerOff() error
	Reboot() error
	PXEBootOnce() error
	Status() Status
}

// Status describes current VM status.
type Status struct {
	PoweredOn bool
}
