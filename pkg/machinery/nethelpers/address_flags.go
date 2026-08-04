// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

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

// Managed masks out the address flags which are managed by Talos.
//
// See [AddressFlagsManaged].
func (flags AddressFlags) Managed() AddressFlags {
	return flags & AddressFlagsManaged
}

// Unmanaged returns the address flags which are managed by the kernel.
//
// See [AddressFlagsManaged].
func (flags AddressFlags) Unmanaged() AddressFlags {
	return flags &^ AddressFlagsManaged
}

// AddressFlagsString converts string representation of flags into AddressFlags.
func AddressFlagsString(s string) (AddressFlags, error) {
	flags := AddressFlags(0)

	for p := range strings.SplitSeq(s, ",") {
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
//
//structprotogen:gen_enum
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

// AddressFlagsManaged is a bitmask of the address flags which are managed (i.e. can be set and enforced) by Talos.
//
// All other flags are managed by the kernel: they are reported as part of the address status, but they
// should never appear in an address spec, and they should be ignored when the desired state of an address is
// compared with the actual state, as the kernel sets and clears them on its own:
//
//   - AddressTemporary (IFA_F_SECONDARY) is set by the kernel for any IPv4 address which shares the subnet with
//     an address already assigned to the link (and for the temporary IPv6 addresses)
//   - AddressTentative and AddressOptimistic are set while IPv6 DAD is in progress, and cleared once it completes
//   - AddressDADFailed is set when IPv6 DAD fails
//   - AddressDeprecated and AddressStablePrivacy are set by the kernel for the addresses it manages itself (SLAAC)
//
// AddressPermanent is kept in the managed set: the kernel derives it from the address lifetime, and Talos always
// assigns addresses without a lifetime, so the flag is stable for the addresses Talos manages.
const AddressFlagsManaged = AddressFlags(
	AddressPermanent |
		AddressNoDAD |
		AddressHome |
		AddressManagementTemp |
		AddressNoPrefixRoute |
		AddressMCAutoJoin,
)
