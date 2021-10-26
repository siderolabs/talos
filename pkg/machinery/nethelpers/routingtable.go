// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=RoutingTable -linecomment

// RoutingTable is a routing table ID.
type RoutingTable uint32

// MarshalYAML implements yaml.Marshaler.
func (table RoutingTable) MarshalYAML() (interface{}, error) {
	return table.String(), nil
}

// RoutingTable constants.
const (
	TableUnspec  RoutingTable = 0   // unspec
	TableDefault RoutingTable = 253 // default
	TableMain    RoutingTable = 254 // main
	TableLocal   RoutingTable = 255 // local
)
