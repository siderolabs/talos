// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import "golang.org/x/sys/unix"

// AttrChanges accumulates mount attribute changes made to one copy of a mount,
// to replay them on another copy: mount attributes never propagate.
type AttrChanges struct {
	Set, Clr uint64
}

// Record records flags being set (on) or cleared; the latest change of a flag wins.
func (c *AttrChanges) Record(on bool, flags uint64) {
	if on {
		c.Set |= flags
		c.Clr &^= flags
	} else {
		c.Clr |= flags
		c.Set &^= flags
	}

	// as in mountSetattr: the atime mode is replaced as a whole
	if (c.Set|c.Clr)&unix.MOUNT_ATTR_NOATIME != 0 {
		c.Clr |= unix.MOUNT_ATTR__ATIME
	}
}

// Empty reports whether no change was recorded.
func (c AttrChanges) Empty() bool {
	return c == AttrChanges{}
}
