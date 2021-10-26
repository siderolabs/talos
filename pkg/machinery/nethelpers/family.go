// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=Family -linecomment

// Family is a network family.
type Family uint8

// MarshalYAML implements yaml.Marshaler.
func (family Family) MarshalYAML() (interface{}, error) {
	return family.String(), nil
}

// Family constants.
const (
	FamilyInet4 Family = 2  // inet4
	FamilyInet6 Family = 10 // inet6
)
