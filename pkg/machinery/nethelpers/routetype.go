// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=RouteType -linecomment

// RouteType is a route type.
type RouteType uint8

// MarshalYAML implements yaml.Marshaler.
func (rt RouteType) MarshalYAML() (interface{}, error) {
	return rt.String(), nil
}

// RouteType constants.
const (
	TypeUnspec      RouteType = iota // unspec
	TypeUnicast                      // unicast
	TypeLocal                        // local
	TypeBroadcast                    // broadcast
	TypeAnycast                      // anycast
	TypeMulticast                    // multicast
	TypeBlackhole                    // blackhole
	TypeUnreachable                  // unreachable
	TypeProhibit                     // prohibit
	TypeThrow                        // throw
	TypeNAT                          // nat
	TypeXResolve                     // xresolve
)
