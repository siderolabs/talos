// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package spike_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/pkg/ntp/internal/spike"
)

func TestSpikeDetector(t *testing.T) {
	for _, test := range []struct {
		name    string
		samples []spike.Sample

		expectedSpikes []bool
	}{
		{
			name: "no spikes",

			samples: []spike.Sample{
				{Offset: 0.01, RTT: 0.01},
				{Offset: 0.05, RTT: 0.01},
				{Offset: 0.03, RTT: 0.01},
				{Offset: 0.01, RTT: 0.01},
				{Offset: -0.01, RTT: 0.01},
				{Offset: -0.02, RTT: 0.03}, // not a spike, just a large RTT
			},

			expectedSpikes: []bool{
				false,
				false,
				false,
				false,
				false,
				false,
			},
		},
		{
			name: "offset spike",

			samples: []spike.Sample{
				{Offset: 0.01, RTT: 0.01},
				{Offset: 0.05, RTT: 0.01},
				{Offset: 0.03, RTT: 0.01},
				{Offset: 0.01, RTT: 0.01},
				{Offset: 0.01, RTT: 0.01},
				{Offset: 0.01, RTT: 0.01},
				{Offset: -0.01, RTT: 0.01},
				{Offset: -0.5, RTT: 0.7}, // spike
			},

			expectedSpikes: []bool{
				false,
				false,
				false,
				false,
				false,
				false,
				false,
				true,
			},
		},
		{
			name: "adjusting to higher RTT",

			samples: []spike.Sample{
				{Offset: 0.01, RTT: 0.01},
				{Offset: 0.05, RTT: 0.01},
				{Offset: 0.03, RTT: 0.01},
				{Offset: 0.01, RTT: 0.01},
				{Offset: -0.01, RTT: 0.01},
				{Offset: 0.01, RTT: 0.01},
				{Offset: -0.01, RTT: 0.01},
				{Offset: -0.5, RTT: 0.7}, // spike
				{Offset: 0.5, RTT: 0.7},  // spike
				{Offset: -0.5, RTT: 0.7}, // spike
				{Offset: 0.5, RTT: 0.7},  // not a spike anymore, filter adjusted itself
				{Offset: -0.5, RTT: 0.7},
				{Offset: 0.01, RTT: 0.01},
			},

			expectedSpikes: []bool{
				false,
				false,
				false,
				false,
				false,
				false,
				false,
				true,
				true,
				true,
				false,
				false,
				false,
			},
		},
		{
			name: "initial ignore",

			samples: []spike.Sample{
				{Offset: 5, RTT: 0.01}, // initial packet is ignored completely
				{Offset: 0.05, RTT: 0.05},
				{Offset: 0.5, RTT: 0.5}, // spike detection kicks in after 4 packets
				{Offset: 0.01, RTT: 0.01},
				{Offset: -0.01, RTT: 0.01},
				{Offset: 0.01, RTT: 0.01},
			},

			expectedSpikes: []bool{
				false,
				false,
				false,
				false,
				false,
				false,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var detector spike.Detector

			for i, sample := range test.samples {
				isSpike := detector.IsSpike(sample)

				assert.Equal(t, test.expectedSpikes[i], isSpike, "unexpected spike: %v (position %d)", test.expectedSpikes[i], i)
			}
		})
	}
}
