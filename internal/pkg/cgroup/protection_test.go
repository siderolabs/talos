// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup //nolint:testpackage

import (
	"testing"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	mib = 1024 * 1024
	gib = 1024 * mib
)

func TestCalculateMemoryProtectionScale(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		totalMemory uint64

		expectedTotal uint64
	}{
		{
			name:          "unknown",
			totalMemory:   0,
			expectedTotal: constants.CgroupTotalReservedMemory,
		},
		{
			name:          "1GiB",
			totalMemory:   875052 * 1024, // MemTotal of a 1GiB QEMU VM
			expectedTotal: 875052 * 1024 / constants.CgroupReservedMemoryMaxFraction,
		},
		{
			name:          "1.5GiB",
			totalMemory:   1536 * mib,
			expectedTotal: 512 * mib,
		},
		{
			name:          "2GiB",
			totalMemory:   2 * gib,
			expectedTotal: constants.CgroupTotalReservedMemory,
		},
		{
			name:          "64GiB",
			totalMemory:   64 * gib,
			expectedTotal: constants.CgroupTotalReservedMemory,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			scale := calculateMemoryProtectionScale(test.totalMemory)

			var total int64

			for _, name := range []string{constants.CgroupInit, constants.CgroupSystem, constants.CgroupPodRuntimeRoot} {
				resources := getCgroupV2Resources(name)
				scaleMemoryProtection(resources, scale)

				total += *resources.Memory.Min
			}

			// rounding down each value might lose a few bytes
			assert.InDelta(t, test.expectedTotal, total, 4)
			assert.LessOrEqual(t, uint64(total), test.expectedTotal)
		})
	}
}

func TestScaleMemoryProtection(t *testing.T) {
	t.Parallel()

	scale := protectionScale{num: 1, den: 2}

	resources := getCgroupV2Resources(constants.CgroupApid)
	scaleMemoryProtection(resources, scale)

	require.NotNil(t, resources.Memory)
	assert.Equal(t, int64(constants.CgroupApidReservedMemory/2), *resources.Memory.Min)
	assert.Equal(t, int64(constants.CgroupApidReservedMemory), *resources.Memory.Low)

	// hard limits are not scaled
	if resources.Memory.Max != nil && *resources.Memory.Max > 0 {
		assert.Equal(t, int64(constants.CgroupApidMaxMemory), *resources.Memory.Max)
	}

	// swap and other values are not touched
	assert.Equal(t, int64(0), *resources.Memory.Swap)

	// no memory settings
	resources = getCgroupV2Resources(constants.CgroupTalosContainersRoot)
	scaleMemoryProtection(resources, scale)
	assert.Nil(t, resources.Memory)

	scaleMemoryProtection(nil, scale)
	scaleMemoryProtection(&cgroup2.Resources{Memory: &cgroup2.Memory{}}, scale)
}
