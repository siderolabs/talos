// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=AddressFlag -linecomment

import (
	"strings"
)

// AddressFlags is a bitmask of AddressFlag.
type AddressFlags uint32

func (flags AddressFlags) String() string {
	var values []string

	for flag := AddressTemporary; flag <= AddressStablePrivacy; flag <<= 1 {
		if (AddressFlag(flags) & flag) == flag {
			values = append(values, flag.String())
		}
	}

	return strings.Join(values, ",")
}

// MarshalYAML implements yaml.Marshaler.
func (flags AddressFlags) MarshalYAML() (interface{}, error) {
	return flags.String(), nil
}

// AddressFlag wraps IFF_* constants.
type AddressFlag uint32

// AddressFlag constants.
const (
	AddressTemporary      AddressFlag = 1 << iota // temporary
	AddressNoDAD                                  // nodad
	AddressOptimistic                             // optimistic
	AddressDADFailed                              // dadfailed
	AddressHome                                   // homeaddress
	AddressDeprecated                             // deprecated
	AddressTentative                              // tentative
	AddressPermanent                              // permanent
	AddressManagementTemp                         // mngmtmpaddr
	AddressNoPrefixRoute                          // noprefixroute
	AddressMCAutoJoin                             // mcautojoin
	AddressStablePrivacy                          // stableprivacy
)
