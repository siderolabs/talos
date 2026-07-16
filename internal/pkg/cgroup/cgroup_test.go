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
