// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clock

import "time"

// SetWallClock is used in test to simulate jumps.
func (d *WallClockJumpDetector) SetWallClock(t time.Time) {
	d.lastWallClock = wallClockOnly(t)
}
