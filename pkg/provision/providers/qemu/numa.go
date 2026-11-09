// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import "math"

func nodeVCPUCount(nanoCPUs int64) int64 {
	return max(1, int64(math.RoundToEven(float64(nanoCPUs)/1000/1000/1000)))
}
