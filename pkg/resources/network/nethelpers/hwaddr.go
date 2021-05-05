// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "net"

// HardwareAddr wraps net.HardwareAddr for YAML marshaling.
type HardwareAddr net.HardwareAddr

// MarshalYAML implements yaml.Marshaler interface.
func (addr HardwareAddr) MarshalYAML() (interface{}, error) {
	return net.HardwareAddr(addr).String(), nil
}
