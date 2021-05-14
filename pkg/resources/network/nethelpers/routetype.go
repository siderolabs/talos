// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "golang.org/x/sys/unix"

//go:generate stringer -type=RouteType -linecomment

// RouteType is a route type.
type RouteType uint8

// MarshalYAML implements yaml.Marshaler.
func (rt RouteType) MarshalYAML() (interface{}, error) {
	return rt.String(), nil
}

// RouteType constants.
const (
	TypeUnspec      RouteType = unix.RTN_UNSPEC      // unspec
	TypeUnicast     RouteType = unix.RTN_UNICAST     // unicast
	TypeLocal       RouteType = unix.RTN_LOCAL       // local
	TypeBroadcast   RouteType = unix.RTN_BROADCAST   // broadcast
	TypeAnycast     RouteType = unix.RTN_ANYCAST     // anycast
	TypeMulticast   RouteType = unix.RTN_MULTICAST   // multicast
	TypeBlackhole   RouteType = unix.RTN_BLACKHOLE   // blackhole
	TypeUnreachable RouteType = unix.RTN_UNREACHABLE // unreachable
	TypeProhibit    RouteType = unix.RTN_PROHIBIT    // prohibit
	TypeThrow       RouteType = unix.RTN_THROW       // throw
	TypeNAT         RouteType = unix.RTN_NAT         // nat
	TypeXResolve    RouteType = unix.RTN_XRESOLVE    // xresolve
)
