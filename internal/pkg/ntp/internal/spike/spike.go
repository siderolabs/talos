// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package spike provides a spike detector for NTP responses.
package spike

import (
	"math"

	"github.com/beevik/ntp"
)

const defaultCapacity = 8

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

// IsSpike returns true if the given sample is a spike.
//
// The sample is added to the detector's internal state.
func (d *Detector) IsSpike(sample Sample) bool {
	if d.samples == nil {
		d.samples = make([]Sample, defaultCapacity)
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

	indexMin := currentIndex

	for i := range d.samples {
		if d.samples[i].RTT == 0 {
			continue
		}

		if d.samples[i].RTT < d.samples[indexMin].RTT {
			indexMin = i
		}
	}

	var j float64

	for i := range d.samples {
		j += math.Pow(d.samples[i].Offset-d.samples[indexMin].Offset, 2)
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

	// This check was specifically removed (while it exists in systemd-timesync),
	// as I don't understand why it's needed (@smira).
	// It seems to give false positives when the RTT and Offset are close to each other,
	// e.g. when NTP server is on the same LAN.
	//
	// if math.Abs(sample.Offset) > d.samples[indexMin].RTT {
	// 	// do not accept anything worse than the maximum possible error of the best sample
	// 	return true
	// }

	// check that diff to the last offset is not more than 3*(observed jitter)
	return math.Abs(sample.Offset-d.samples[currentIndex].Offset) > 3*jitter
}

// Jitter returns the current jitter.
func (d *Detector) Jitter() float64 {
	return d.samplesJitter
}
