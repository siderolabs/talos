// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	beevikntp "github.com/beevik/ntp"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/ntp"
	"github.com/siderolabs/talos/internal/pkg/timex"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

type NTPSuite struct {
	suite.Suite

	clockLock        sync.Mutex
	systemClock      time.Time
	clockAdjustments []time.Duration

	failingServer int
	spikyServer   int
}

func TestNTPSuite(t *testing.T) {
	suite.Run(t, new(NTPSuite))
}

func (suite *NTPSuite) SetupSuite() {
	// disable RTC clock
	ntp.RTCClockInitialize.Do(func() {})
}

func (suite *NTPSuite) SetupTest() {
	suite.systemClock = time.Now().UTC()
	suite.clockAdjustments = nil
	suite.failingServer = 0
}

func (suite *NTPSuite) getSystemClock() time.Time {
	suite.clockLock.Lock()
	defer suite.clockLock.Unlock()

	return suite.systemClock
}

func (suite *NTPSuite) adjustSystemClock(val *unix.Timex) (status timex.State, err error) {
	suite.clockLock.Lock()
	defer suite.clockLock.Unlock()

	if val.Modes&unix.ADJ_OFFSET == unix.ADJ_OFFSET {
		suite.T().Logf("adjustment by %s", time.Duration(val.Offset)*time.Nanosecond)
		suite.clockAdjustments = append(suite.clockAdjustments, time.Duration(val.Offset)*time.Nanosecond)
	} else {
		suite.T().Logf("set clock by %s", time.Duration(val.Time.Sec)*time.Second+time.Duration(val.Time.Usec)*time.Nanosecond)
		suite.systemClock = suite.systemClock.Add(time.Duration(val.Time.Sec)*time.Second + time.Duration(val.Time.Usec)*time.Nanosecond)
	}

	return
}

func (suite *NTPSuite) fakeQuery(host string) (resp *beevikntp.Response, err error) {
	switch host {
	case "127.0.0.1": // error
		return nil, errors.New("no response")
	case "127.0.0.2": // invalid response
		resp = &beevikntp.Response{}

		suite.Require().Error(resp.Validate())

		return resp, nil
	case "127.0.0.3": // adjust +1ms
		resp = &beevikntp.Response{
			Stratum:       1,
			Time:          suite.systemClock,
			ReferenceTime: suite.systemClock,
			ClockOffset:   time.Millisecond,
			RTT:           time.Millisecond / 2,
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	case "127.0.0.4": // adjust +2ms
		resp = &beevikntp.Response{
			Stratum:       1,
			Time:          suite.systemClock,
			ReferenceTime: suite.systemClock,
			ClockOffset:   2 * time.Millisecond,
			RTT:           time.Millisecond / 2,
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	case "127.0.0.5": // adjust +2*epoch
		resp = &beevikntp.Response{
			Stratum:       1,
			Time:          suite.systemClock,
			ReferenceTime: suite.systemClock,
			ClockOffset:   ntp.EpochLimit * 2,
			RTT:           time.Millisecond,
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	case "127.0.0.6": // adjust 1ms/fail alternating
		suite.failingServer++

		if suite.failingServer%2 == 0 {
			return nil, errors.New("failed this time")
		}

		resp = &beevikntp.Response{
			Stratum:       1,
			Time:          suite.systemClock,
			ReferenceTime: suite.systemClock,
			ClockOffset:   time.Millisecond,
			RTT:           time.Millisecond / 2,
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	case "127.0.0.7": // server with spikes
		suite.spikyServer++

		if suite.spikyServer%5 == 4 {
			resp = &beevikntp.Response{
				Stratum:       1,
				Time:          suite.systemClock,
				ReferenceTime: suite.systemClock,
				ClockOffset:   time.Second,
				RTT:           2 * time.Second,
			}
		} else {
			resp = &beevikntp.Response{
				Stratum:       1,
				Time:          suite.systemClock,
				ReferenceTime: suite.systemClock,
				ClockOffset:   time.Millisecond,
				RTT:           time.Millisecond / 2,
			}
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	default:
		return nil, fmt.Errorf("unknown host %q", host)
	}
}

func (suite *NTPSuite) TestSync() {
	syncer := ntp.NewSyncer(zaptest.NewLogger(suite.T()).With(zap.String("controller", "ntp")), []string{constants.DefaultNTPServer})

	syncer.AdjustTime = suite.adjustSystemClock
	syncer.CurrentTime = suite.getSystemClock

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		syncer.Run(ctx)
	}()

	select {
	case <-syncer.Synced():
	case <-time.After(10 * time.Second):
		suite.Assert().Fail("time sync timeout")
	}

	cancel()

	wg.Wait()
}

func (suite *NTPSuite) TestSyncContinuous() {
	syncer := ntp.NewSyncer(zaptest.NewLogger(suite.T()).With(zap.String("controller", "ntp")), []string{"127.0.0.3"})

	syncer.AdjustTime = suite.adjustSystemClock
	syncer.CurrentTime = suite.getSystemClock
	syncer.NTPQuery = suite.fakeQuery

	syncer.MinPoll = time.Second
	syncer.MaxPoll = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		syncer.Run(ctx)
	}()

	select {
	case <-syncer.Synced():
	case <-time.After(10 * time.Second):
		suite.Assert().Fail("time sync timeout")
	}

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			suite.clockLock.Lock()
			defer suite.clockLock.Unlock()

			if len(suite.clockAdjustments) < 3 {
				return retry.ExpectedErrorf("not enough syncs")
			}

			return nil
		}),
	)

	cancel()

	wg.Wait()
}

func (suite *NTPSuite) TestSyncWithSpikes() {
	syncer := ntp.NewSyncer(zaptest.NewLogger(suite.T()).With(zap.String("controller", "ntp")), []string{"127.0.0.7"})

	syncer.AdjustTime = suite.adjustSystemClock
	syncer.CurrentTime = suite.getSystemClock
	syncer.NTPQuery = suite.fakeQuery

	syncer.MinPoll = time.Second
	syncer.MaxPoll = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		syncer.Run(ctx)
	}()

	select {
	case <-syncer.Synced():
	case <-time.After(10 * time.Second):
		suite.Assert().Fail("time sync timeout")
	}

	suite.Assert().NoError(
		retry.Constant(12*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			suite.clockLock.Lock()
			defer suite.clockLock.Unlock()

			if len(suite.clockAdjustments) < 6 {
				return retry.ExpectedErrorf("not enough syncs")
			}

			for _, adj := range suite.clockAdjustments {
				// 1s spike should be filtered out
				suite.Assert().Equal(time.Millisecond, adj)
			}

			return nil
		}),
	)

	cancel()

	wg.Wait()
}

func (suite *NTPSuite) TestSyncChangeTimeservers() {
	syncer := ntp.NewSyncer(zaptest.NewLogger(suite.T()).With(zap.String("controller", "ntp")), []string{"127.0.0.1"})

	syncer.AdjustTime = suite.adjustSystemClock
	syncer.CurrentTime = suite.getSystemClock

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		syncer.Run(ctx)
	}()

	select {
	case <-syncer.Synced():
		suite.Assert().Fail("unexpected sync")
	case <-time.After(3 * time.Second):
	}

	syncer.SetTimeServers([]string{constants.DefaultNTPServer})

	select {
	case <-syncer.Synced():
	case <-time.After(10 * time.Second):
		suite.Assert().Fail("time sync timeout")
	}

	cancel()

	wg.Wait()
}

func (suite *NTPSuite) TestSyncIterateTimeservers() {
	syncer := ntp.NewSyncer(zaptest.NewLogger(suite.T()).With(zap.String("controller", "ntp")), []string{"127.0.0.1", "127.0.0.2", "127.0.0.3", "127.0.0.4"})

	syncer.AdjustTime = suite.adjustSystemClock
	syncer.CurrentTime = suite.getSystemClock
	syncer.NTPQuery = suite.fakeQuery

	syncer.MinPoll = time.Second
	syncer.MaxPoll = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		syncer.Run(ctx)
	}()

	select {
	case <-syncer.Synced():
	case <-time.After(10 * time.Second):
		suite.Assert().Fail("time sync timeout")
	}

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			suite.clockLock.Lock()
			defer suite.clockLock.Unlock()

			if len(suite.clockAdjustments) < 3 {
				return retry.ExpectedErrorf("not enough syncs")
			}

			return nil
		}),
	)

	cancel()

	wg.Wait()

	// should always sync with 127.0.0.3, and never switch to 127.0.0.4
	for i := range 3 {
		suite.Assert().Equal(time.Millisecond, suite.clockAdjustments[i])
	}
}

func (suite *NTPSuite) TestSyncEpochChange() {
	syncer := ntp.NewSyncer(zaptest.NewLogger(suite.T()).With(zap.String("controller", "ntp")), []string{"127.0.0.5"})

	syncer.AdjustTime = suite.adjustSystemClock
	syncer.CurrentTime = suite.getSystemClock
	syncer.NTPQuery = suite.fakeQuery

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		syncer.Run(ctx)
	}()

	select {
	case <-syncer.Synced():
	case <-time.After(10 * time.Second):
		suite.Assert().Fail("time sync timeout")
	}

	select {
	case <-syncer.EpochChange():
	case <-time.After(10 * time.Second):
		suite.Assert().Fail("epoch change timeout")
	}

	cancel()

	wg.Wait()

	suite.Assert().Greater(-time.Since(suite.systemClock), ntp.EpochLimit)
}

func (suite *NTPSuite) TestSyncSwitchTimeservers() {
	syncer := ntp.NewSyncer(zaptest.NewLogger(suite.T()).With(zap.String("controller", "ntp")), []string{"127.0.0.6", "127.0.0.4"})

	syncer.AdjustTime = suite.adjustSystemClock
	syncer.CurrentTime = suite.getSystemClock
	syncer.NTPQuery = suite.fakeQuery

	syncer.MinPoll = time.Second
	syncer.MaxPoll = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		syncer.Run(ctx)
	}()

	select {
	case <-syncer.Synced():
	case <-time.After(10 * time.Second):
		suite.Assert().Fail("time sync timeout")
	}

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			suite.clockLock.Lock()
			defer suite.clockLock.Unlock()

			if len(suite.clockAdjustments) < 3 {
				return retry.ExpectedErrorf("not enough syncs")
			}

			return nil
		}),
	)

	cancel()

	wg.Wait()

	// should start sync with 127.0.0.6, then switch to 127.0.0.4
	suite.Assert().Equal(time.Millisecond, suite.clockAdjustments[0])

	for i := 1; i < 3; i++ {
		suite.Assert().Equal(2*time.Millisecond, suite.clockAdjustments[i])
	}
}
