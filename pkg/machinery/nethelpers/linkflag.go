// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate enumer -type=LinkFlag -linecomment -text

import (
	"strings"
)

// LinkFlags is a bitmask of LinkFlags.
type LinkFlags uint32

func (flags LinkFlags) String() string {
	var values []string

	for flag := LinkUp; flag <= LinkEcho; flag <<= 1 {
		if (LinkFlag(flags) & flag) == flag {
			values = append(values, flag.String())
		}
	}

	return strings.Join(values, ",")
}

// LinkFlagsString parses string representation of LinkFlags.
func LinkFlagsString(s string) (LinkFlags, error) {
	flags := LinkFlags(0)

	for _, p := range strings.Split(s, ",") {
		flag, err := LinkFlagString(p)
		if err != nil {
			return flags, err
		}

		flags |= LinkFlags(flag)
	}

	return flags, nil
}

// MarshalText implements text.Marshaler.
func (flags LinkFlags) MarshalText() ([]byte, error) {
	return []byte(flags.String()), nil
}

// UnmarshalText implements text.Unmarshaler.
func (flags *LinkFlags) UnmarshalText(b []byte) error {
	var err error

	*flags, err = LinkFlagsString(string(b))

	return err
}

// LinkFlag wraps IFF_* constants.
type LinkFlag uint32

// LinkFlag constants.
const (
	LinkUp           LinkFlag = 1 << iota // UP
	LinkBroadcast                         // BROADCAST
	LinkDebug                             // DEBUG
	LinkLoopback                          // LOOPBACK
	LinkPointToPoint                      // POINTTOPOINT
	LinkNoTrailers                        // NOTRAILERS
	LinkRunning                           // RUNNING
	LinkNoArp                             // NOARP
	LinkPromisc                           // PROMISC
	LinkAllMulti                          // ALLMULTI
	LinkMaster                            // MASTER
	LinkSlave                             // SLAVE
	LinkMulticase                         // MULTICAST
	LinkPortsel                           // PORTSEL
	LinKAutoMedia                         // AUTOMEDIA
	LinkDynamic                           // DYNAMIC
	LinkLowerUp                           // LOWER_UP
	LinkDormant                           // DORMANT
	LinkEcho                              // ECHO
)
