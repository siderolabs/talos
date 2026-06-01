// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package startup

import (
	"log"
	"runtime"
)

// LimitMaxProcs limits the GOMAXPROCS to the number specified.
func LimitMaxProcs(maxProcs int) {
	curProcs := runtime.GOMAXPROCS(0)

	if curProcs > maxProcs {
		runtime.GOMAXPROCS(maxProcs)

		log.Printf("limited GOMAXPROCS to %d", maxProcs)
	}
}
