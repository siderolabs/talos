// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup

import (
	"runtime"
	"sync"

	"github.com/google/cadvisor/utils/sysfs"
	"github.com/google/cadvisor/utils/sysinfo"
)

var availableCPUCores = sync.OnceValue(func() int {
	_, cores, err := sysinfo.GetNodesInfo(sysfs.NewRealSysFs())
	if err != nil || cores < 1 {
		return runtime.NumCPU()
	}

	return cores
})

// MilliCores represents a CPU value in milli-cores.
type MilliCores uint

// AvailableMilliCores returns the number of available CPU cores in milli-cores.
func AvailableMilliCores() MilliCores {
	return MilliCores(availableCPUCores()) * 1000
}

// CPUShare represents a CPU share value.
type CPUShare uint64

// MilliCoresToShares converts milli-cores to CPU shares.
func MilliCoresToShares(milliCores MilliCores) CPUShare {
	return CPUShare(milliCores) * 1024 / 1000
}

// SharesToCPUWeight converts CPU shares to CPU weight.
func SharesToCPUWeight(shares CPUShare) uint64 {
	return uint64((((shares - 2) * 9999) / 262142) + 1)
}

// MillicoresToCPUWeight converts milli-cores to CPU weight.
//
// It limits millicores to available CPU cores.
func MillicoresToCPUWeight(requested MilliCores) uint64 {
	requested = min(requested, AvailableMilliCores())

	return SharesToCPUWeight(MilliCoresToShares(requested))
}
