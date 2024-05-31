// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "github.com/mdlayher/ethtool"

// Port wraps ethtool.Port for YAML marshaling.
type Port ethtool.Port

// Possible Port type values.
//
//structprotogen:gen_enum
const (
	TwistedPair  Port = Port(ethtool.TwistedPair)
	AUI          Port = Port(ethtool.AUI)
	MII          Port = Port(ethtool.MII)
	Fibre        Port = Port(ethtool.Fibre) //nolint:misspell
	BNC          Port = Port(ethtool.BNC)
	DirectAttach Port = Port(ethtool.DirectAttach)
	None         Port = Port(ethtool.None)
	Other        Port = Port(ethtool.Other)
)
