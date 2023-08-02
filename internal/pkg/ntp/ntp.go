// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ntp provides a time sync client via SNTP protocol.
package ntp

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/bits"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/u-root/u-root/pkg/rtc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/timex"
)

// Syncer performs time sync via NTP on schedule.
type Syncer struct {
	logger *zap.Logger

	timeServersMu  sync.Mutex
	timeServers    []string
	lastSyncServer string

	timeSyncNotified bool
	timeSynced       chan struct{}

	restartSyncCh chan struct{}
	epochChangeCh chan struct{}

	firstSync bool

	packetCount   int64
	samples       []sample
	samplesIdx    int
	samplesJitter float64

	MinPoll, MaxPoll, RetryPoll time.Duration

	// these functions are overridden in tests for mocking support
	CurrentTime CurrentTimeFunc
	NTPQuery    QueryFunc
	AdjustTime  AdjustTimeFunc
}

const sampleCount = 8

type sample struct {
	offset, rtt float64 // in seconds
}

// NewSyncer creates new Syncer with default configuration.
func NewSyncer(logger *zap.Logger, timeServers []string) *Syncer {
	syncer := &Syncer{
		logger: logger,

		timeServers: append([]string(nil), timeServers...),
		timeSynced:  make(chan struct{}),

		restartSyncCh: make(chan struct{}, 1),
		epochChangeCh: make(chan struct{}, 1),

		firstSync: true,

		samples: make([]sample, sampleCount),

		MinPoll:   MinAllowablePoll,
		MaxPoll:   MaxAllowablePoll,
		RetryPoll: RetryPoll,

		CurrentTime: time.Now,
		NTPQuery:    ntp.Query,
		AdjustTime:  timex.Adjtimex,
	}

	return syncer
}

// Synced returns a channel which is closed when time is in sync.
func (syncer *Syncer) Synced() <-chan struct{} {
	return syncer.timeSynced
}

// EpochChange returns a channel which receives a value each time jumps more than EpochLimit.
func (syncer *Syncer) EpochChange() <-chan struct{} {
	return syncer.epochChangeCh
}

func (syncer *Syncer) getTimeServers() []string {
	syncer.timeServersMu.Lock()
	defer syncer.timeServersMu.Unlock()

	return syncer.timeServers
}

func (syncer *Syncer) getLastSyncServer() string {
	syncer.timeServersMu.Lock()
	defer syncer.timeServersMu.Unlock()

	return syncer.lastSyncServer
}

func (syncer *Syncer) setLastSyncServer(lastSyncServer string) {
	syncer.timeServersMu.Lock()
	defer syncer.timeServersMu.Unlock()

	syncer.lastSyncServer = lastSyncServer
}

// SetTimeServers sets the list of time servers to use.
func (syncer *Syncer) SetTimeServers(timeServers []string) {
	syncer.timeServersMu.Lock()
	defer syncer.timeServersMu.Unlock()

	if reflect.DeepEqual(timeServers, syncer.timeServers) {
		return
	}

	syncer.timeServers = append([]string(nil), timeServers...)
	syncer.lastSyncServer = ""

	syncer.restartSync()
}

func (syncer *Syncer) restartSync() {
	select {
	case syncer.restartSyncCh <- struct{}{}:
	default:
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}

	return d
}

func (syncer *Syncer) spikeDetector(resp *ntp.Response) bool {
	syncer.packetCount++

	if syncer.packetCount == 1 {
		// ignore first packet
		return false
	}

	var currentIndex int

	currentIndex, syncer.samplesIdx = syncer.samplesIdx, (syncer.samplesIdx+1)%sampleCount

	syncer.samples[syncer.samplesIdx].offset = resp.ClockOffset.Seconds()
	syncer.samples[syncer.samplesIdx].rtt = resp.RTT.Seconds()

	jitter := syncer.samplesJitter

	indexMin := currentIndex

	for i := range syncer.samples {
		if syncer.samples[i].rtt == 0 {
			continue
		}

		if syncer.samples[i].rtt < syncer.samples[indexMin].rtt {
			indexMin = i
		}
	}

	var j float64

	for i := range syncer.samples {
		j += math.Pow(syncer.samples[i].offset-syncer.samples[indexMin].offset, 2)
	}

	syncer.samplesJitter = math.Sqrt(j / (sampleCount - 1))

	if absDuration(resp.ClockOffset) > resp.RTT {
		// always accept clock offset if that is larger than rtt
		return false
	}

	if syncer.packetCount < 4 {
		// need more samples to make a decision
		return false
	}

	if absDuration(resp.ClockOffset).Seconds() > syncer.samples[indexMin].rtt {
		// do not accept anything worse than the maximum possible error of the best sample
		return true
	}

	return math.Abs(resp.ClockOffset.Seconds()-syncer.samples[currentIndex].offset) > 3*jitter
}

// Run runs the sync process.
//
// Run is usually run in a goroutine.
// When context is canceled, sync process aborts.
//
//nolint:gocyclo,cyclop
func (syncer *Syncer) Run(ctx context.Context) {
	RTCClockInitialize.Do(func() {
		var err error

		RTCClock, err = rtc.OpenRTC()
		if err != nil {
			syncer.logger.Error("failure opening RTC, ignored", zap.Error(err))
		}
	})

	pollInterval := time.Duration(0)

	for {
		lastSyncServer, resp, err := syncer.query(ctx)
		if err != nil {
			return
		}

		spike := false

		if resp != nil && resp.Validate() == nil {
			spike = syncer.spikeDetector(resp)
		}

		switch {
		case resp == nil:
			// if no response was ever received, consider doing short sleep to retry sooner as it's not Kiss-o-Death response
			pollInterval = syncer.RetryPoll
		case pollInterval == 0:
			// first sync
			pollInterval = syncer.MinPoll
		case err != nil:
			// error encountered, don't change the poll interval
		case !spike && absDuration(resp.ClockOffset) > ExpectedAccuracy:
			// huge offset, retry sync with minimum interval
			pollInterval = syncer.MinPoll
		case absDuration(resp.ClockOffset) < ExpectedAccuracy*100/25: // *0.25
			// clock offset is within 25% of expected accuracy, increase poll interval
			if pollInterval < syncer.MaxPoll {
				pollInterval *= 2
			}
		case spike || absDuration(resp.ClockOffset) > ExpectedAccuracy*100/75: // *0.75
			// spike was detected or clock offset is too large, decrease poll interval
			if pollInterval > syncer.MinPoll {
				pollInterval /= 2
			}
		}

		if resp != nil && pollInterval < syncer.MinPoll {
			// set poll interval to at least min poll if there was any response
			pollInterval = syncer.MinPoll
		}

		syncer.logger.Debug("sample stats",
			zap.Duration("jitter", time.Duration(syncer.samplesJitter*float64(time.Second))),
			zap.Duration("poll_interval", pollInterval),
			zap.Bool("spike", spike),
		)

		if resp != nil && resp.Validate() == nil && !spike {
			err = syncer.adjustTime(resp.ClockOffset, resp.Leap, lastSyncServer, pollInterval)

			if err == nil {
				if !syncer.timeSyncNotified {
					// successful first time sync, notify about it
					close(syncer.timeSynced)

					syncer.timeSyncNotified = true
				}
			} else {
				syncer.logger.Error("error adjusting time", zap.Error(err))
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-syncer.restartSyncCh:
			// time servers got changed, restart the loop immediately
		case <-time.After(pollInterval):
		}
	}
}

func (syncer *Syncer) query(ctx context.Context) (lastSyncServer string, resp *ntp.Response, err error) {
	lastSyncServer = syncer.getLastSyncServer()
	failedServer := ""

	if lastSyncServer != "" {
		resp, err = syncer.queryServer(lastSyncServer)
		if err != nil {
			syncer.logger.Error(fmt.Sprintf("ntp query error with server %q", lastSyncServer), zap.Error(err))

			failedServer = lastSyncServer
			lastSyncServer = ""
			err = nil
		}
	}

	if lastSyncServer == "" {
		var serverList []string

		serverList, err = syncer.resolveServers(ctx)
		if err != nil {
			return lastSyncServer, resp, err
		}

		for _, server := range serverList {
			if server == failedServer {
				// skip server which failed in previous sync to avoid sending requests with short interval
				continue
			}

			select {
			case <-ctx.Done():
				return lastSyncServer, resp, ctx.Err()
			case <-syncer.restartSyncCh:
				return lastSyncServer, resp, nil
			default:
			}

			resp, err = syncer.queryServer(server)
			if err != nil {
				syncer.logger.Error(fmt.Sprintf("ntp query error with server %q", server), zap.Error(err))
				err = nil
			} else {
				syncer.setLastSyncServer(server)
				lastSyncServer = server

				break
			}
		}
	}

	return lastSyncServer, resp, err
}

func (syncer *Syncer) resolveServers(ctx context.Context) ([]string, error) {
	var serverList []string

	for _, server := range syncer.getTimeServers() {
		ips, err := net.LookupIP(server)
		if err != nil {
			syncer.logger.Warn(fmt.Sprintf("failed looking up %q, ignored", server), zap.Error(err))
		}

		for _, ip := range ips {
			serverList = append(serverList, ip.String())
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return serverList, nil
}

func (syncer *Syncer) queryServer(server string) (*ntp.Response, error) {
	resp, err := syncer.NTPQuery(server)
	if err != nil {
		return nil, err
	}

	syncer.logger.Debug("NTP response",
		zap.Duration("clock_offset", resp.ClockOffset),
		zap.Duration("rtt", resp.RTT),
		zap.Uint8("leap", uint8(resp.Leap)),
		zap.Uint8("stratum", resp.Stratum),
		zap.Duration("precision", resp.Precision),
		zap.Duration("root_delay", resp.RootDelay),
		zap.Duration("root_dispersion", resp.RootDispersion),
		zap.Duration("root_distance", resp.RootDistance),
	)

	if err = resp.Validate(); err != nil {
		return resp, err
	}

	return resp, err
}

// log2i returns 0 for v == 0 and v == 1.
func log2i(v uint64) int {
	if v == 0 {
		return 0
	}

	return 63 - bits.LeadingZeros64(v)
}

// adjustTime adds an offset to the current time.
//
//nolint:gocyclo
func (syncer *Syncer) adjustTime(offset time.Duration, leapSecond ntp.LeapIndicator, server string, nextPollInterval time.Duration) error {
	var (
		buf  bytes.Buffer
		req  unix.Timex
		jump bool
	)

	if offset < -AdjustTimeLimit || offset > AdjustTimeLimit {
		jump = true

		fmt.Fprintf(&buf, "adjusting time (jump) by %s via %s", offset, server)

		req = unix.Timex{
			Modes: unix.ADJ_SETOFFSET | unix.ADJ_NANO | unix.ADJ_STATUS | unix.ADJ_MAXERROR | unix.ADJ_ESTERROR,
			Time: unix.Timeval{
				Sec:  int64(offset / time.Second),
				Usec: int64(offset / time.Nanosecond % time.Second),
			},
			Maxerror: 0,
			Esterror: 0,
		}

		// kernel wants tv_usec to be positive
		if req.Time.Usec < 0 {
			req.Time.Sec--
			req.Time.Usec += int64(time.Second / time.Nanosecond)
		}
	} else {
		fmt.Fprintf(&buf, "adjusting time (slew) by %s via %s", offset, server)

		pollSeconds := uint64(nextPollInterval / time.Second)
		log2iPollSeconds := log2i(pollSeconds)

		req = unix.Timex{
			Modes:    unix.ADJ_OFFSET | unix.ADJ_NANO | unix.ADJ_STATUS | unix.ADJ_TIMECONST | unix.ADJ_MAXERROR | unix.ADJ_ESTERROR,
			Offset:   int64(offset / time.Nanosecond),
			Status:   unix.STA_PLL,
			Maxerror: 0,
			Esterror: 0,
			Constant: int64(log2iPollSeconds) - 4,
		}
	}

	switch leapSecond { //nolint:exhaustive
	case ntp.LeapAddSecond:
		req.Status |= unix.STA_INS
	case ntp.LeapDelSecond:
		req.Status |= unix.STA_DEL
	}

	logLevel := zapcore.DebugLevel

	if jump {
		logLevel = zapcore.InfoLevel
	}

	state, err := syncer.AdjustTime(&req)

	fmt.Fprintf(&buf, ", state %s, status %s", state, timex.Status(req.Status))

	if err != nil {
		logLevel = zapcore.WarnLevel

		fmt.Fprintf(&buf, ", error was %s", err)
	}

	if syncer.firstSync && logLevel == zapcore.DebugLevel {
		// promote first sync to info level
		syncer.firstSync = false

		logLevel = zapcore.InfoLevel
	}

	if ce := syncer.logger.Check(logLevel, buf.String()); ce != nil {
		ce.Write()
	}

	syncer.logger.Debug("adjtime state",
		zap.Int64("constant", req.Constant),
		zap.Duration("offset", time.Duration(req.Offset)),
		zap.Int64("freq_offset", req.Freq),
		zap.Int64("freq_offset_ppm", req.Freq/65536),
	)

	if err == nil {
		if offset < -EpochLimit || offset > EpochLimit {
			// notify about epoch change
			select {
			case syncer.epochChangeCh <- struct{}{}:
			default:
			}
		}

		if jump {
			if RTCClock != nil {
				if rtcErr := RTCClock.Set(time.Now().Add(offset)); rtcErr != nil {
					syncer.logger.Error("error syncing RTC", zap.Error(rtcErr))
				} else {
					syncer.logger.Info("synchronized RTC with system clock")
				}
			}
		}
	}

	return err
}
