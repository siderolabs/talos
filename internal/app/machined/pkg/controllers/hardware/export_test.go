// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

// SetWriteSysfs replaces how the controller writes sysfs attributes.
func (c *PCIDriverRebindController) SetWriteSysfs(write func(path string, contents []byte) error) {
	c.writeSysfs = write
}
