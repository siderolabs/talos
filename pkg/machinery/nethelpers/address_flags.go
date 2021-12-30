// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate enumer -type=AddressFlag -linecomment -text

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

// AddressFlagsString converts string representation of flags into AddressFlags.
func AddressFlagsString(s string) (AddressFlags, error) {
	flags := AddressFlags(0)

	for _, p := range strings.Split(s, ",") {
		flag, err := AddressFlagString(p)
		if err != nil {
			return flags, err
		}

		flags |= AddressFlags(flag)
	}

	return flags, nil
}

// MarshalText implements text.Marshaler.
func (flags AddressFlags) MarshalText() ([]byte, error) {
	return []byte(flags.String()), nil
}

// UnmarshalText implements text.Unmarshaler.
func (flags *AddressFlags) UnmarshalText(b []byte) error {
	var err error

	*flags, err = AddressFlagsString(string(b))

	return err
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
