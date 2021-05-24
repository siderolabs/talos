// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=AddressFlag -linecomment -output addressflag_string_linux.go

import (
	"strings"

	"golang.org/x/sys/unix"
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
	AddressTemporary      AddressFlag = unix.IFA_F_TEMPORARY      // temporary
	AddressNoDAD          AddressFlag = unix.IFA_F_NODAD          // nodad
	AddressOptimistic     AddressFlag = unix.IFA_F_OPTIMISTIC     // optimistic
	AddressDADFailed      AddressFlag = unix.IFA_F_DADFAILED      // dadfailed
	AddressHome           AddressFlag = unix.IFA_F_HOMEADDRESS    // homeaddress
	AddressDeprecated     AddressFlag = unix.IFA_F_DEPRECATED     // deprecated
	AddressTentative      AddressFlag = unix.IFA_F_TENTATIVE      // tentative
	AddressPermanent      AddressFlag = unix.IFA_F_PERMANENT      // permanent
	AddressManagementTemp AddressFlag = unix.IFA_F_MANAGETEMPADDR // mngmtmpaddr
	AddressNoPrefixRoute  AddressFlag = unix.IFA_F_NOPREFIXROUTE  // noprefixroute
	AddressMCAutoJoin     AddressFlag = unix.IFA_F_MCAUTOJOIN     // mcautojoin
	AddressStablePrivacy  AddressFlag = unix.IFA_F_STABLE_PRIVACY // stableprivacy
)
