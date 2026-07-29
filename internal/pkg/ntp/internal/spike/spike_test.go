// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package spike_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
				{Offset: -0.02, RTT: 0.025}, // not a spike, just a large RTT: 2.5x the best sample is not enough
				//                              asymmetry to explain a 20ms offset on a 10ms path
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
			// The best-sample error bound only applies once the round-trip is inflated enough
			// for one-directional congestion to account for the offset, so the very same
			// offset is believed on a mildly jittery path and rejected on a congested one.
			name: "RTT inflation gate",

			samples: []spike.Sample{
				// fill the whole window with a 5ms path, so that the best sample is 5ms
				{Offset: 0.011, RTT: 0.005},
				{Offset: 0.013, RTT: 0.005},
				{Offset: 0.011, RTT: 0.005},
				{Offset: 0.013, RTT: 0.005},
				{Offset: 0.011, RTT: 0.005},
				{Offset: 0.013, RTT: 0.005},
				{Offset: 0.011, RTT: 0.005},
				{Offset: 0.013, RTT: 0.005},
				{Offset: 0.011, RTT: 0.005},
				{Offset: 0.012, RTT: 0.014}, // not a spike: 2.8x the best sample, believed
				{Offset: 0.012, RTT: 0.016}, // spike: 3.2x the best sample, congestion can explain it
			},

			expectedSpikes: []bool{
				false,
				false,
				false,
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
			// The best sample in the window bounds how large an offset can be believed, so
			// adaptation to a permanently slower path takes one full window: the low-RTT
			// samples have to age out before the new regime becomes the best sample.
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
				{Offset: 0.5, RTT: 0.7},  // spike
				{Offset: -0.5, RTT: 0.7}, // spike
				{Offset: 0.5, RTT: 0.7},  // spike
				{Offset: -0.5, RTT: 0.7}, // spike
				{Offset: 0.5, RTT: 0.7},  // not a spike anymore, the window is full of the new regime
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
				true,
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

// TestSpikeDetectorReset covers switching to a server on a slower path: without a reset the
// samples of the old path gate the new one until they age out of the window.
func TestSpikeDetectorReset(t *testing.T) {
	var detector spike.Detector

	// a LAN server on a 1ms path
	for range 4 {
		require.False(t, detector.IsSpike(spike.Sample{Offset: 0.0002, RTT: 0.001}))
	}

	// the LAN server goes away, the next one in the list sits behind a 60ms path: its samples
	// are measured against a best sample which describes a network path that is no longer used
	assert.True(t, detector.IsSpike(spike.Sample{Offset: 0.03, RTT: 0.06}))

	detector.Reset()

	assert.Zero(t, detector.Jitter())

	for range 4 {
		assert.False(t, detector.IsSpike(spike.Sample{Offset: 0.03, RTT: 0.06}))
	}
}

// TestSpikeDetectorAsymmetricDelay replays NTP polls captured from a node whose upstream
// path suffers ponctual congestion.
//
// Asymmetric delay can bias the offset calculation by roughly half the excess RTT, so a
// congested poll looks like a large clock error. Accepting one slews that error into the
// clock, and the next poll then reports -- and corrects -- an error the daemon introduced
// itself, so the clock sawtooths by tens of milliseconds while each individual adjustment
// still looks plausible.
//
// Rejecting these samples cannot rely on the jitter estimate, because the jitter is itself
// inflated by the congestion (below it reaches 60ms while the path is only 5ms wide).
func TestSpikeDetectorAsymmetricDelay(t *testing.T) {
	var detector spike.Detector

	for _, sample := range []spike.Sample{
		{Offset: 0.0004, RTT: 0.0055},
		{Offset: -0.0005, RTT: 0.0054},
		{Offset: 0.0003, RTT: 0.0055},
		{Offset: -0.0004, RTT: 0.0053},
		{Offset: 0.0005, RTT: 0.0055},
		{Offset: -0.0003, RTT: 0.0054},
		{Offset: 0.0004, RTT: 0.0055},
		{Offset: -0.0005, RTT: 0.0054},
	} {
		require.False(t, detector.IsSpike(sample), "steady state must not be flagged")
	}

	for _, sample := range []struct {
		name   string
		sample spike.Sample

		expectedSpike bool
	}{
		// offset ~= (RTT - 5.5ms)/2 throughout: asymmetric delay, not a real clock error
		{"asymmetric delay, 98ms RTT", spike.Sample{Offset: 0.046067, RTT: 0.098445}, true},
		// a clean poll in between is believed, as it should be
		{"clean poll", spike.Sample{Offset: -0.000647, RTT: 0.005533}, false},
		{"asymmetric delay, 67ms RTT", spike.Sample{Offset: 0.031782, RTT: 0.067470}, true},
		// |offset| > RTT: too large to be explained by asymmetry on a 5ms path, so the
		// measurement is trustworthy and is applied
		{"real offset on a fast path", spike.Sample{Offset: -0.046647, RTT: 0.005483}, false},
		{"asymmetric delay, 201ms RTT", spike.Sample{Offset: 0.104740, RTT: 0.201382}, true},
		{"asymmetric delay, 110ms RTT", spike.Sample{Offset: 0.063678, RTT: 0.110155}, true},
		{"real offset on a fast path, again", spike.Sample{Offset: -0.084529, RTT: 0.005420}, false},
		{"asymmetric delay, 32ms RTT", spike.Sample{Offset: 0.027669, RTT: 0.032066}, true},
	} {
		assert.Equal(t, sample.expectedSpike, detector.IsSpike(sample.sample), "unexpected verdict for %q", sample.name)
	}
}
