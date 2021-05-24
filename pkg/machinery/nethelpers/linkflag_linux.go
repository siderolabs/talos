// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=LinkFlag -linecomment -output linkflag_string_linux.go

import (
	"strings"

	"golang.org/x/sys/unix"
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
	LinkUp           LinkFlag = unix.IFF_UP          // UP
	LinkBroadcast    LinkFlag = unix.IFF_BROADCAST   // BROADCAST
	LinkDebug        LinkFlag = unix.IFF_DEBUG       // DEBUG
	LinkLoopback     LinkFlag = unix.IFF_LOOPBACK    // LOOPBACK
	LinkPointToPoint LinkFlag = unix.IFF_POINTOPOINT // POINTTOPOINT
	LinkRunning      LinkFlag = unix.IFF_RUNNING     // RUNNING
	LinkNoArp        LinkFlag = unix.IFF_NOARP       // NOARP
	LinkPromisc      LinkFlag = unix.IFF_PROMISC     // PROMISC
	LinkNoTrailers   LinkFlag = unix.IFF_NOTRAILERS  // NOTRAILERS
	LinkAllMulti     LinkFlag = unix.IFF_ALLMULTI    // ALLMULTI
	LinkMaster       LinkFlag = unix.IFF_MASTER      // MASTER
	LinkSlave        LinkFlag = unix.IFF_SLAVE       // SLAVE
	LinkMulticase    LinkFlag = unix.IFF_MULTICAST   // MULTICAST
	LinkPortsel      LinkFlag = unix.IFF_PORTSEL     // PORTSEL
	LinKAutoMedia    LinkFlag = unix.IFF_AUTOMEDIA   // AUTOMEDIA
	LinkDynamic      LinkFlag = unix.IFF_DYNAMIC     // DYNAMIC
	LinkLowerUp      LinkFlag = unix.IFF_LOWER_UP    // LOWER_UP
	LinkDormant      LinkFlag = unix.IFF_DORMANT     // DORMANT
	LinkEcho         LinkFlag = unix.IFF_ECHO        // ECHO
)
