// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=RouteFlag -linecomment -output routeflag_string_linux.go

import (
	"strings"

	"golang.org/x/sys/unix"
)

// RouteFlags is a bitmask of RouteFlag.
type RouteFlags uint32

func (flags RouteFlags) String() string {
	var values []string

	for flag := RouteNotify; flag <= RouteTrap; flag <<= 1 {
		if (RouteFlag(flags) & flag) == flag {
			values = append(values, flag.String())
		}
	}

	return strings.Join(values, ",")
}

// Equal tests for RouteFlags equality ignoring flags not managed by this implementation.
func (flags RouteFlags) Equal(other RouteFlags) bool {
	return (flags & RouteFlags(RouteFlagsMask)) == (other & RouteFlags(RouteFlagsMask))
}

// MarshalYAML implements yaml.Marshaler.
func (flags RouteFlags) MarshalYAML() (interface{}, error) {
	return flags.String(), nil
}

// RouteFlag wraps RTM_F_* constants.
type RouteFlag uint32

// RouteFlag constants.
const (
	RouteNotify      RouteFlag = unix.RTM_F_NOTIFY       // notify
	RouteCloned      RouteFlag = unix.RTM_F_CLONED       // cloned
	RouteEqualize    RouteFlag = unix.RTM_F_EQUALIZE     // equalize
	RoutePrefix      RouteFlag = unix.RTM_F_PREFIX       // prefix
	RouteLookupTable RouteFlag = unix.RTM_F_LOOKUP_TABLE // lookup_table
	RouteFIBMatch    RouteFlag = unix.RTM_F_FIB_MATCH    // fib_match
	RouteOffload     RouteFlag = unix.RTM_F_OFFLOAD      // offload
	RouteTrap        RouteFlag = unix.RTM_F_TRAP         // trap
)

// RouteFlagsMask is a supported set of flags to manage.
const RouteFlagsMask = RouteNotify | RouteCloned | RouteEqualize | RoutePrefix
