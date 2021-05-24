// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "golang.org/x/sys/unix"

//go:generate stringer -type=RoutingTable -linecomment -output routingtable_string_linux.go

// RoutingTable is a routing table ID.
type RoutingTable uint32

// MarshalYAML implements yaml.Marshaler.
func (table RoutingTable) MarshalYAML() (interface{}, error) {
	return table.String(), nil
}

// RoutingTable constants.
const (
	TableUnspec  RoutingTable = unix.RT_TABLE_UNSPEC  // unspec
	TableDefault RoutingTable = unix.RT_TABLE_DEFAULT // default
	TableMain    RoutingTable = unix.RT_TABLE_MAIN    // main
	TableLocal   RoutingTable = unix.RT_TABLE_LOCAL   // local
)
