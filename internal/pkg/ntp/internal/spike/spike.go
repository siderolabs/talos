// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package spike provides a spike detector for NTP responses.
package spike

import (
	"math"

	"github.com/beevik/ntp"
)

// SampleCount is the number of samples the detector keeps, i.e. the size of the window a
// change in the network path or in the clock takes to work its way through the detector.
const SampleCount = 8

// rttInflationFactor is how much the round-trip time of a sample has to exceed the best
// round-trip time in the window before the best-sample error bound is applied to it.
//
// One-directional congestion biases the offset by at most half of the excess delay, so the
// congestion can only explain an offset larger than the best round-trip time when
// (RTT-minRTT)/2 > minRTT, i.e. when RTT > 3*minRTT. Below that ratio the offset cannot be
// an artifact of the extra delay, so the measurement is believed: this is what keeps
// ordinary round-trip wobble (e.g. an NTP server on the same LAN) from being filtered out.
const rttInflationFactor = 3

// Sample is a single NTP response sample.
type Sample struct {
	Offset, RTT float64 // in seconds
}

// SampleFromNTPResponse converts an NTP response to a Sample.
func SampleFromNTPResponse(resp *ntp.Response) Sample {
	return Sample{
		Offset: resp.ClockOffset.Seconds(),
		RTT:    resp.RTT.Seconds(),
	}
}

// Detector detects spikes in NTP response samples.
//
// Zero value is ready to use.
type Detector struct {
	packetCount   int64
	samples       []Sample
	samplesIdx    int
	samplesJitter float64
}

// bestSample returns the index of the sample with the lowest round-trip time, i.e. the one
// which was the least affected by the network.
//
// Slots which were never filled in hold a zero round-trip time and are skipped, including
// when the search starts on one of them.
func (d *Detector) bestSample(startIndex int) int {
	indexMin := startIndex

	for i := range d.samples {
		if d.samples[i].RTT <= 0 {
			continue
		}

		if d.samples[indexMin].RTT <= 0 || d.samples[i].RTT < d.samples[indexMin].RTT {
			indexMin = i
		}
	}

	return indexMin
}

// IsSpike returns true if the given sample is a spike.
//
// The sample is added to the detector's internal state.
func (d *Detector) IsSpike(sample Sample) bool {
	if d.samples == nil {
		d.samples = make([]Sample, SampleCount)
	}

	d.packetCount++

	if d.packetCount == 1 {
		// ignore first packet
		return false
	}

	var currentIndex int

	currentIndex, d.samplesIdx = d.samplesIdx, (d.samplesIdx+1)%len(d.samples)

	d.samples[d.samplesIdx] = sample

	jitter := d.samplesJitter

	indexMin := d.bestSample(currentIndex)

	// the round-trip time of the best sample in the window, zero if the window holds no
	// usable sample yet (slots which were never filled in have a zero RTT)
	bestRTT := max(d.samples[indexMin].RTT, 0)

	var j float64

	for i := range d.samples {
		offsetDiff := d.samples[i].Offset - d.samples[indexMin].Offset
		j += offsetDiff * offsetDiff
	}

	d.samplesJitter = math.Sqrt(j / (float64(len(d.samples)) - 1))

	if math.Abs(sample.Offset) > sample.RTT {
		// always accept clock offset if that is larger than rtt
		return false
	}

	if d.packetCount < 4 {
		// need more samples to make a decision
		return false
	}

	// Do not accept anything worse than the maximum possible error of the best sample, but
	// only once the round-trip of this sample is inflated enough for asymmetric delay to
	// account for the offset (see rttInflationFactor).
	//
	// This is what one-directional congestion looks like: the extra delay sits on one leg of
	// the round-trip and biases the offset by roughly half of it, so a congested poll reports
	// a large clock error while still staying below the |offset| > RTT check above.
	if bestRTT > 0 && sample.RTT > rttInflationFactor*bestRTT && math.Abs(sample.Offset) > bestRTT {
		return true
	}

	// check that diff to the last offset is not more than 3*(observed jitter)
	return math.Abs(sample.Offset-d.samples[currentIndex].Offset) > 3*jitter
}

// Jitter returns the current jitter.
func (d *Detector) Jitter() float64 {
	return d.samplesJitter
}

// Reset drops the sample history.
//
// The samples describe a single clock observed over a single network path, so they stop
// being meaningful once either changes: when the time server is switched the round-trip
// times describe the old path, and when the clock is stepped the offsets describe the old
// clock. In both cases the detector has to start over instead of measuring the new regime
// against the old one.
func (d *Detector) Reset() {
	*d = Detector{}
}
