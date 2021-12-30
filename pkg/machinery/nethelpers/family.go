// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate enumer -type=Family -linecomment -text

// Family is a network family.
type Family uint8

// Family constants.
const (
	FamilyInet4 Family = 2  // inet4
	FamilyInet6 Family = 10 // inet6
)
