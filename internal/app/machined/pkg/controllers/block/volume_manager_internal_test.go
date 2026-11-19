// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestIsFailedUserVolumeProvisioning(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		label        string
		tearingDown  bool
		phase        block.VolumePhase
		preFailPhase block.VolumePhase

		expected bool
	}{
		{
			name:         "inline data volume failed before mount",
			label:        block.InlineDataVolumeLabel,
			tearingDown:  true,
			phase:        block.VolumePhaseFailed,
			preFailPhase: block.VolumePhaseWaiting,
			expected:     true,
		},
		{
			name:         "inline data volume failed after mount",
			label:        block.InlineDataVolumeLabel,
			tearingDown:  true,
			phase:        block.VolumePhaseFailed,
			preFailPhase: block.VolumePhasePrepared,
			expected:     false,
		},
		{
			name:         "inline data volume not tearing down",
			label:        block.InlineDataVolumeLabel,
			phase:        block.VolumePhaseFailed,
			preFailPhase: block.VolumePhaseWaiting,
			expected:     false,
		},
		{
			name:         "system volume",
			tearingDown:  true,
			phase:        block.VolumePhaseFailed,
			preFailPhase: block.VolumePhaseWaiting,
			expected:     false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			volumeConfig := block.NewVolumeConfig(block.NamespaceName, "volume")
			if test.label != "" {
				volumeConfig.Metadata().Labels().Set(test.label, "")
			}

			volumeStatus := block.NewVolumeStatus(block.NamespaceName, "volume")
			volumeStatus.TypedSpec().Phase = test.phase
			volumeStatus.TypedSpec().PreFailPhase = test.preFailPhase

			assert.Equal(t, test.expected, isFailedUserVolumeProvisioning(test.tearingDown, volumeConfig, volumeStatus))
		})
	}
}
