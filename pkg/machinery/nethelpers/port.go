// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "github.com/mdlayher/ethtool"

// Port wraps ethtool.Port for YAML marshaling.
type Port ethtool.Port

// MarshalText implements text.Marshaler interface.
func (port Port) MarshalText() ([]byte, error) {
	return []byte(ethtool.Port(port).String()), nil
}
