// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

// SetWriteSysfs replaces how the controller writes sysfs attributes.
func (c *PCIDriverRebindController) SetWriteSysfs(write func(path string, contents []byte) error) {
	c.writeSysfs = write
}

// SetDeviceAccess replaces how the controller checks how device files are used.
func (c *PCIDriverRebindController) SetDeviceAccess(devices deviceAccess) {
	c.devices = devices
}

// DeviceNumber is the number of a device file.
type DeviceNumber = deviceNumber

// NewDeviceNumber returns the number of a block or character device file.
func NewDeviceNumber(rdev uint64, block bool) DeviceNumber {
	return deviceNumber{rdev: rdev, block: block}
}

// HostDeviceNumber returns the number of the device file at a path, as read on the host.
func HostDeviceNumber(path string) (DeviceNumber, bool, error) {
	return hostDeviceAccess{}.DeviceNumber(path)
}
