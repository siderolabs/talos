// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

// MDLevel describes the RAID level of an MD (software RAID) array.
type MDLevel int

// MD RAID levels.
//
//structprotogen:gen_enum
const (
	MDLevelRAID1 MDLevel = iota // raid1
)

// Mdadm returns the numeric RAID level mdadm expects on its --level flag.
func (l MDLevel) Mdadm() int {
	switch l {
	case MDLevelRAID1:
		return 1
	default:
		return 1
	}
}
