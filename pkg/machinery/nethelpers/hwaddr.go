// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"bytes"
	"encoding/hex"
	"net"
)

// HardwareAddr wraps net.HardwareAddr for YAML marshaling.
type HardwareAddr net.HardwareAddr

// MarshalText implements text.Marshaler interface.
func (addr HardwareAddr) MarshalText() ([]byte, error) {
	return []byte(net.HardwareAddr(addr).String()), nil
}

// UnmarshalText implements text.Unmarshaler interface.
func (addr *HardwareAddr) UnmarshalText(b []byte) error {
	rawHex := bytes.ReplaceAll(b, []byte(":"), []byte(""))
	dstLen := hex.DecodedLen(len(rawHex))

	dst := make([]byte, dstLen)

	n, err := hex.Decode(dst, rawHex)
	if err != nil {
		return err
	}

	*addr = HardwareAddr(dst[:n])

	return nil
}

func (addr HardwareAddr) String() string {
	return net.HardwareAddr(addr).String()
}
