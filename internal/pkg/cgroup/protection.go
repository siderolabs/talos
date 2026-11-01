// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup

import (
	"sync"

	"github.com/containerd/cgroups/v3/cgroup2"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// protectionScale is a ratio num/den applied to memory.min and memory.low values.
type protectionScale struct {
	num, den uint64
}

// noScale keeps memory protection values as is.
var noScale = protectionScale{num: 1, den: 1}

// memoryProtectionScale returns the scale for memory protection values based on the total system memory.
var memoryProtectionScale = sync.OnceValue(func() protectionScale {
	var info unix.Sysinfo_t

	if err := unix.Sysinfo(&info); err != nil {
		return noScale
	}

	return calculateMemoryProtectionScale(info.Totalram * uint64(info.Unit))
})

// calculateMemoryProtectionScale scales down memory protection so that it doesn't exceed
// a fraction of the total memory.
//
// Memory protection covers page cache as well, so if most of the memory is protected,
// the kernel can't reclaim page cache and invokes the OOM killer instead.
func calculateMemoryProtectionScale(totalMemory uint64) protectionScale {
	maxProtected := totalMemory / constants.CgroupReservedMemoryMaxFraction

	if totalMemory == 0 || maxProtected >= constants.CgroupTotalReservedMemory {
		return noScale
	}

	return protectionScale{num: maxProtected, den: constants.CgroupTotalReservedMemory}
}

func (s protectionScale) apply(v *int64) {
	if v == nil || *v <= 0 || s == noScale {
		return
	}

	*v = int64(uint64(*v) * s.num / s.den)
}

// scaleMemoryProtection applies the scale to memory.min and memory.low.
//
// Hard limits (memory.max) are not affected.
func scaleMemoryProtection(resources *cgroup2.Resources, scale protectionScale) {
	if resources == nil || resources.Memory == nil {
		return
	}

	scale.apply(resources.Memory.Min)
	scale.apply(resources.Memory.Low)
}
