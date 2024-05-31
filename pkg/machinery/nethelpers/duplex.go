// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "github.com/mdlayher/ethtool"

// Duplex wraps ethtool.Duplex for YAML marshaling.
type Duplex ethtool.Duplex

// Possible Duplex type values.
//
//structprotogen:gen_enum
const (
	Half    Duplex = Duplex(ethtool.Half)    // Half
	Full    Duplex = Duplex(ethtool.Full)    // Full
	Unknown Duplex = Duplex(ethtool.Unknown) // Unknown
)
