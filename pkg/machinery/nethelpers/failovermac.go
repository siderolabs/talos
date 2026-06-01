// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"fmt"
	"strconv"
)

// FailOverMAC is a MAC failover mode.
type FailOverMAC uint8

// FailOverMAC constants.
//
//structprotogen:gen_enum
const (
	FailOverMACNone   FailOverMAC = iota // none
	FailOverMACActive                    // active
	FailOverMACFollow                    // follow
)

// FailOverMACByName parses FailOverMac.
func FailOverMACByName(f string) (FailOverMAC, error) {
	switch f {
	case "", "none":
		return FailOverMACNone, nil
	case "active":
		return FailOverMACActive, nil
	case "follow":
		return FailOverMACFollow, nil
	default:
		return 0, fmt.Errorf("invalid fail_over_mac value %v", f)
	}
}

// MarshalText implements encoding.TextMarshaler.
func (f FailOverMAC) MarshalText() ([]byte, error) {
	return []byte(f.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
//
// There is some old historical reason we don't use enumer's -text method to automatically
// generate the MarshalText/UnmarshalText methods - so we have to implement UnmarshalText manually
// to support both name and numeric representation for backward compatibility.
func (f *FailOverMAC) UnmarshalText(text []byte) error {
	parsed, err := FailOverMACByName(string(text))
	if err != nil {
		// for legacy compatibility try to parse as a number
		out, parseErr := strconv.ParseInt(string(text), 10, 8)
		if parseErr == nil && out >= int64(FailOverMACNone) && out <= int64(FailOverMACFollow) {
			*f = FailOverMAC(out)

			return nil
		}

		return err
	}

	*f = parsed

	return nil
}
