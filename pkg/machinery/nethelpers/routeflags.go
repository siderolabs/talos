// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate enumer -type=RouteFlag -linecomment -text

import (
	"strings"
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

// RouteFlagsString parses string representation into RouteFlags.
func RouteFlagsString(s string) (RouteFlags, error) {
	flags := RouteFlags(0)

	for _, p := range strings.Split(s, ",") {
		flag, err := RouteFlagString(p)
		if err != nil {
			return flags, err
		}

		flags |= RouteFlags(flag)
	}

	return flags, nil
}

// Equal tests for RouteFlags equality ignoring flags not managed by this implementation.
func (flags RouteFlags) Equal(other RouteFlags) bool {
	return (flags & RouteFlags(RouteFlagsMask)) == (other & RouteFlags(RouteFlagsMask))
}

// MarshalText implements text.Marshaler.
func (flags RouteFlags) MarshalText() ([]byte, error) {
	return []byte(flags.String()), nil
}

// UnmarshalText implements text.Unmarshaler.
func (flags *RouteFlags) UnmarshalText(b []byte) error {
	var err error

	*flags, err = RouteFlagsString(string(b))

	return err
}

// RouteFlag wraps RTM_F_* constants.
type RouteFlag uint32

// RouteFlag constants.
const (
	RouteNotify      RouteFlag = 256 << iota // notify
	RouteCloned                              // cloned
	RouteEqualize                            // equalize
	RoutePrefix                              // prefix
	RouteLookupTable                         // lookup_table
	RouteFIBMatch                            // fib_match
	RouteOffload                             // offload
	RouteTrap                                // trap
)

// RouteFlagsMask is a supported set of flags to manage.
const RouteFlagsMask = RouteNotify | RouteCloned | RouteEqualize | RoutePrefix
