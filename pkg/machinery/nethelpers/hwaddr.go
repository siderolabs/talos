// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "net"

// HardwareAddr wraps net.HardwareAddr for YAML marshaling.
type HardwareAddr net.HardwareAddr

// MarshalText implements text.Marshaler interface.
func (addr HardwareAddr) MarshalText() ([]byte, error) {
	return []byte(net.HardwareAddr(addr).String()), nil
}

// UnmarshalText implements text.Unmarshaler interface.
func (addr *HardwareAddr) UnmarshalText(b []byte) error {
	mac, err := net.ParseMAC(string(b))
	if err != nil {
		return err
	}

	*addr = HardwareAddr(mac)

	return nil
}

func (addr HardwareAddr) String() string {
	return net.HardwareAddr(addr).String()
}
