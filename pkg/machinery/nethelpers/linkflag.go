// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=LinkFlag -linecomment

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

// MarshalYAML implements yaml.Marshaler.
func (flags LinkFlags) MarshalYAML() (interface{}, error) {
	return flags.String(), nil
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
