// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package smart_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/smart"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestComputeNVMeStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		details  block.DiskHealthNVMeDetails
		expected block.DiskHealthStatusValue
	}{
		{
			name:     "healthy",
			details:  block.DiskHealthNVMeDetails{},
			expected: block.DiskHealthStatusValueHealthy,
		},
		{
			name:     "critical warning set",
			details:  block.DiskHealthNVMeDetails{CriticalWarning: 1},
			expected: block.DiskHealthStatusValueCritical,
		},
		{
			name:     "media errors",
			details:  block.DiskHealthNVMeDetails{MediaAndDataIntegrityErrors: 5},
			expected: block.DiskHealthStatusValueCritical,
		},
		{
			name:     "high percentage used",
			details:  block.DiskHealthNVMeDetails{PercentageUsed: 95},
			expected: block.DiskHealthStatusValueWarning,
		},
		{
			name:     "percentage used at boundary",
			details:  block.DiskHealthNVMeDetails{PercentageUsed: 90},
			expected: block.DiskHealthStatusValueHealthy,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := smart.ComputeNVMeStatus(&tc.details)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestComputeATAStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		details  block.DiskHealthATADetails
		expected block.DiskHealthStatusValue
	}{
		{
			name:     "healthy",
			details:  block.DiskHealthATADetails{},
			expected: block.DiskHealthStatusValueHealthy,
		},
		{
			name:     "critical - offline uncorrectable",
			details:  block.DiskHealthATADetails{OfflineUncorrectableCount: 1},
			expected: block.DiskHealthStatusValueCritical,
		},
		{
			name:     "critical - reallocated plus offline",
			details:  block.DiskHealthATADetails{ReallocatedSectorCount: 5, OfflineUncorrectableCount: 1},
			expected: block.DiskHealthStatusValueCritical,
		},
		{
			name:     "warning - pending sectors",
			details:  block.DiskHealthATADetails{CurrentPendingSectorCount: 2},
			expected: block.DiskHealthStatusValueWarning,
		},
		{
			name:     "warning - reallocated only",
			details:  block.DiskHealthATADetails{ReallocatedSectorCount: 3},
			expected: block.DiskHealthStatusValueWarning,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := smart.ComputeATAStatus(&tc.details)
			assert.Equal(t, tc.expected, result)
		})
	}
}
