// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "github.com/mdlayher/ethtool"

// WOLMode wraps ethtool.WOLMode for YAML marshaling.
type WOLMode int

// Constants copied from ethtool to provide Stringer interface.
//
//structprotogen:gen_enum
const (
	WOLModePhy         WOLMode = WOLMode(ethtool.PHY)         // phy
	WOLModeUnicast     WOLMode = WOLMode(ethtool.Unicast)     // unicast
	WOLModeMulticast   WOLMode = WOLMode(ethtool.Multicast)   // multicast
	WOLModeBroadcast   WOLMode = WOLMode(ethtool.Broadcast)   // broadcast
	WOLModeMagic       WOLMode = WOLMode(ethtool.Magic)       // magic
	WOLModeMagicSecure WOLMode = WOLMode(ethtool.MagicSecure) // magicsecure
	WOLModeFilter      WOLMode = WOLMode(ethtool.Filter)      // filter
)

// WOLModeMin is the minimum valid WOLMode.
const WOLModeMin = WOLModePhy

// WOLModeMax is the maximum valid WOLMode.
const WOLModeMax = WOLModeFilter
