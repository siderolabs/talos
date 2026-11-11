// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup //nolint:testpackage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestPodRuntimeRootMemoryProtection(t *testing.T) {
	t.Parallel()

	resources := getCgroupV2Resources(constants.CgroupPodRuntimeRoot)
	require.NotNil(t, resources.Memory)
	require.NotNil(t, resources.Memory.Min)
	require.NotNil(t, resources.Memory.Low)
	require.Equal(t, int64(constants.CgroupPodRuntimeRootReservedMemory), *resources.Memory.Min)
	require.Equal(t, int64(constants.CgroupPodRuntimeRootSoftReservedMemory), *resources.Memory.Low)
}

// An unset cpu.weight is the kernel default of 100, worth about 2.5 cores on the millicore scale,
// so every cgroup Talos creates next to its weighted siblings carries a weight of its own.
func TestCPUWeightDefaults(t *testing.T) {
	t.Parallel()

	for name, millicores := range map[string]int{
		constants.CgroupSystemDebug: constants.CgroupSystemDebugMillicores,
		constants.CgroupExtensions:  constants.CgroupExtensionsMillicores,
		constants.CgroupInstaller:   constants.CgroupInstallerMillicores,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resources := getCgroupV2Resources(name)
			require.NotNil(t, resources.CPU)
			require.NotNil(t, resources.CPU.Weight)
			require.Equal(t, MillicoresToCPUWeight(MilliCores(millicores)), *resources.CPU.Weight)
		})
	}
}
