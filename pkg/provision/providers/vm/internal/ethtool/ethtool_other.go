// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

package ethtool

// TXOff disables TX checksum offload on specified interface.
//
// Not applicable on non-linux systems.
func TXOff(name string) error {
	return nil
}
