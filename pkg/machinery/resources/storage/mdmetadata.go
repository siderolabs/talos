// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

// MDMetadata describes the on-disk metadata format of an MD (software RAID) array.
type MDMetadata int

// MD metadata formats.
//
// The zero value is MDMetadata10 so an unset config defaults to 1.0.
//
//structprotogen:gen_enum
const (
	MDMetadata10 MDMetadata = iota // 1.0
	MDMetadata12                   // 1.2
)

// Mdadm returns the value mdadm expects on its --metadata flag.
func (m MDMetadata) Mdadm() string {
	return m.String()
}
