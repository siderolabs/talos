// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ntp provides a time sync client via SNTP protocol.
package ntp

import (
	"bytes"
	"context"
	"fmt"
	"math/bits"
	"net"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/u-root/u-root/pkg/rtc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/ntp/internal/spike"
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

	spikeDetector spike.Detector

	MinPoll, MaxPoll, RetryPoll time.Duration

	// these functions are overridden in tests for mocking support
	CurrentTime CurrentTimeFunc
	NTPQuery    QueryFunc
	AdjustTime  AdjustTimeFunc
}

// Measurement is a struct containing correction data based on a time request.
type Measurement struct {
	ClockOffset time.Duration
	Leap        ntp.LeapIndicator
	Spike       bool
}

// NewSyncer creates new Syncer with default configuration.
func NewSyncer(logger *zap.Logger, timeServers []string) *Syncer {
	syncer := &Syncer{
		logger: logger,

		timeServers: slices.Clone(timeServers),
		timeSynced:  make(chan struct{}),

		restartSyncCh: make(chan struct{}, 1),
		epochChangeCh: make(chan struct{}, 1),

		firstSync: true,

		spikeDetector: spike.Detector{},

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

	syncer.timeServers = slices.Clone(timeServers)
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

func (syncer *Syncer) isSpike(resp *ntp.Response) bool {
	return syncer.spikeDetector.IsSpike(spike.SampleFromNTPResponse(resp))
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
		if resp != nil {
			spike = resp.Spike
		}

		switch {
		case resp == nil:
			// if no response was ever received, consider doing short sleep to retry sooner as it's not Kiss-o-Death response
			pollInterval = syncer.RetryPoll
		case pollInterval == 0:
			// first sync
			pollInterval = syncer.MinPoll
		case !spike && absDuration(resp.ClockOffset) > ExpectedAccuracy:
			// huge offset, retry sync with minimum interval
			pollInterval = syncer.MinPoll
		case absDuration(resp.ClockOffset) < ExpectedAccuracy*25/100: // *0.25
			// clock offset is within 25% of expected accuracy, increase poll interval
			if pollInterval < syncer.MaxPoll {
				pollInterval *= 2
			}
		case spike || absDuration(resp.ClockOffset) > ExpectedAccuracy*75/100: // *0.75
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
			zap.Duration("jitter", time.Duration(syncer.spikeDetector.Jitter()*float64(time.Second))),
			zap.Duration("poll_interval", pollInterval),
			zap.Bool("spike", spike),
		)

		if resp != nil && !spike {
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

func (syncer *Syncer) query(ctx context.Context) (lastSyncServer string, measurement *Measurement, err error) {
	lastSyncServer = syncer.getLastSyncServer()
	failedServer := ""

	if lastSyncServer != "" {
		measurement, err = syncer.queryServer(lastSyncServer)
		if err != nil {
			syncer.logger.Error(fmt.Sprintf("time query error with server %q", lastSyncServer), zap.Error(err))

			failedServer = lastSyncServer
			lastSyncServer = ""
			err = nil
		}
	}

	if lastSyncServer == "" {
		var serverList []string

		serverList, err = syncer.resolveServers(ctx)
		if err != nil {
			return lastSyncServer, measurement, err
		}

		for _, server := range serverList {
			if server == failedServer {
				// skip server which failed in previous sync to avoid sending requests with short interval
				continue
			}

			select {
			case <-ctx.Done():
				return lastSyncServer, measurement, ctx.Err()
			case <-syncer.restartSyncCh:
				return lastSyncServer, measurement, nil
			default:
			}

			measurement, err = syncer.queryServer(server)
			if err != nil {
				syncer.logger.Error(fmt.Sprintf("time query error with server %q", server), zap.Error(err))
				err = nil
			} else {
				syncer.setLastSyncServer(server)
				lastSyncServer = server

				break
			}
		}
	}

	return lastSyncServer, measurement, err
}

func (syncer *Syncer) isPTPDevice(server string) bool {
	return strings.HasPrefix(server, "/dev/")
}

func (syncer *Syncer) resolveServers(ctx context.Context) ([]string, error) {
	var serverList []string

	for _, server := range syncer.getTimeServers() {
		if syncer.isPTPDevice(server) {
			serverList = append(serverList, server)
		} else {
			ips, err := net.LookupIP(server)
			if err != nil {
				syncer.logger.Warn(fmt.Sprintf("failed looking up %q, ignored", server), zap.Error(err))
			}

			for _, ip := range ips {
				serverList = append(serverList, ip.String())
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return serverList, nil
}

func (syncer *Syncer) queryServer(server string) (*Measurement, error) {
	if syncer.isPTPDevice(server) {
		return syncer.queryPTP(server)
	}

	return syncer.queryNTP(server)
}

func (syncer *Syncer) queryPTP(server string) (*Measurement, error) {
	phc, err := os.Open(server)
	if err != nil {
		return nil, err
	}

	defer phc.Close() //nolint:errcheck

	// From clock_gettime(2):
	//
	// Using  the  appropriate  macros,  open file descriptors may be converted into clock IDs and passed to clock_gettime(), clock_settime(), and clock_adjtime(2).  The
	// following example shows how to convert a file descriptor into a dynamic clock ID.
	//
	// 	#define CLOCKFD 3
	// 	#define FD_TO_CLOCKID(fd)   ((~(clockid_t) (fd) << 3) | CLOCKFD)

	clockid := int32(3 | (^phc.Fd() << 3))

	var ts unix.Timespec

	err = unix.ClockGettime(clockid, &ts)
	if err != nil {
		return nil, err
	}

	offset := time.Until(time.Unix(ts.Sec, ts.Nsec))
	syncer.logger.Debug("PTP clock",
		zap.Duration("clock_offset", offset),
		zap.Int64("sec", ts.Sec),
		zap.Int64("nsec", ts.Nsec),
		zap.String("device", server),
	)

	meas := &Measurement{
		ClockOffset: offset,
		Leap:        0,
		Spike:       false,
	}

	return meas, err
}

func (syncer *Syncer) queryNTP(server string) (*Measurement, error) {
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

	validationError := resp.Validate()
	if validationError != nil {
		return nil, validationError
	}

	return &Measurement{
		ClockOffset: resp.ClockOffset,
		Leap:        resp.Leap,
		Spike:       syncer.isSpike(resp),
	}, nil
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
