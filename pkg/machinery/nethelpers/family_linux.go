// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "golang.org/x/sys/unix"

//go:generate stringer -type=Family -linecomment -output family_string_linux.go

// Family is a network family.
type Family uint8

// MarshalYAML implements yaml.Marshaler.
func (family Family) MarshalYAML() (interface{}, error) {
	return family.String(), nil
}

// Family constants.
const (
	FamilyInet4 Family = unix.AF_INET  // inet4
	FamilyInet6 Family = unix.AF_INET6 // inet6
)
