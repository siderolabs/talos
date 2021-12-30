// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate enumer -type=RouteType -linecomment -text

// RouteType is a route type.
type RouteType uint8

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
