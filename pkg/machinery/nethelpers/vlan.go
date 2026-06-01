// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"crypto/sha256"
	"fmt"
)

const maxLinkNameLength = 15

// VLANLinkName builds a VLAN link name out of the base device name and VLAN ID.
//
// The function takes care of the maximum length of the link name.
func VLANLinkName(base string, vlanID uint16) string {
	// VLAN ID is actually 12-bit, so the allowed values are 0-4095.
	// In ".%d" format, vlanID can be up to 5 characters long.
	if len(base)+5 <= maxLinkNameLength {
		return fmt.Sprintf("%s.%d", base, vlanID)
	}

	// If the base name is too long, we need to truncate it, but simply
	// truncating might lead to ambiguous link name, so take some hash of the original
	// name.
	prefix := base[:4]

	hash := sha256.Sum256([]byte(base))

	return fmt.Sprintf("%s%x.%d", prefix, hash[:(maxLinkNameLength-len(prefix)-5)/2], vlanID)
}
