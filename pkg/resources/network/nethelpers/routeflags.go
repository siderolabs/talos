// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=RouteFlag -linecomment

import (
	"strings"

	"golang.org/x/sys/unix"
)

// RouteFlags is a bitmask of RouteFlag.
type RouteFlags uint32

func (flags RouteFlags) String() string {
	var values []string

	for flag := RouteNotify; flag <= RoutePrefix; flag <<= 1 {
		if (RouteFlag(flags) & flag) == flag {
			values = append(values, flag.String())
		}
	}

	return strings.Join(values, ",")
}

// MarshalYAML implements yaml.Marshaler.
func (flags RouteFlags) MarshalYAML() (interface{}, error) {
	return flags.String(), nil
}

// RouteFlag wraps RTM_F_* constants.
type RouteFlag uint32

// RouteFlag constants.
const (
	RouteNotify   RouteFlag = unix.RTM_F_NOTIFY   // notify
	RouteCloned   RouteFlag = unix.RTM_F_CLONED   // cloned
	RouteEqualize RouteFlag = unix.RTM_F_EQUALIZE // equalize
	RoutePrefix   RouteFlag = unix.RTM_F_PREFIX   // prefix
)
