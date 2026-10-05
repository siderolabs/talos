// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kernel

import (
	"fmt"
	"math"
)

// pageCounterMax is the kernel's PAGE_COUNTER_MAX for 64-bit targets: LONG_MAX / PAGE_SIZE
// (include/linux/page_counter.h). A page counter at this value is reported as "max", and
// page_counter_memparse clamps every larger request to it. Talos ships 64-bit kernels only;
// the 32-bit definition (LONG_MAX) does not apply.
func pageCounterMax(pageSize int) uint64 {
	return math.MaxInt64 / uint64(pageSize)
}

// NormalizeMemoryLimit rounds a finite limit down to the target page size.
//
// Rejecting PAGE_COUNTER_MAX before writing prevents a finite request from becoming unlimited.
func NormalizeMemoryLimit(limit uint64, pageSize int) (uint64, error) {
	if pageSize <= 0 {
		return 0, fmt.Errorf("invalid page size %d", pageSize)
	}

	if limit > math.MaxInt64 {
		return 0, fmt.Errorf("memory limit %d bytes exceeds the maximum of %d bytes", limit, uint64(math.MaxInt64))
	}

	pages := limit / uint64(pageSize)

	if pages == 0 {
		return 0, fmt.Errorf("memory limit %d bytes is smaller than the page size of %d bytes", limit, pageSize)
	}

	if pages >= pageCounterMax(pageSize) {
		return 0, fmt.Errorf("memory limit %d bytes is treated as unlimited by the kernel (limit must be below %d bytes)", limit, pageCounterMax(pageSize)*uint64(pageSize))
	}

	return pages * uint64(pageSize), nil
}
