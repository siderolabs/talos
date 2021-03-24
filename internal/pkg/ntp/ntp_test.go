// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"syscall"
	"testing"
	"time"

	beevikntp "github.com/beevik/ntp"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/pkg/ntp"
	"github.com/talos-systems/talos/internal/pkg/timex"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

type NTPSuite struct {
	suite.Suite

	clockLock        sync.Mutex
	systemClock      time.Time
	clockAdjustments []time.Duration

	failingServer int
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

func (suite *NTPSuite) setSystemClock(timeval *syscall.Timeval) error {
	suite.clockLock.Lock()
	defer suite.clockLock.Unlock()

	suite.systemClock = time.Unix(timeval.Sec, timeval.Usec)

	return nil
}

func (suite *NTPSuite) getSystemClock() time.Time {
	suite.clockLock.Lock()
	defer suite.clockLock.Unlock()

	return suite.systemClock
}

func (suite *NTPSuite) adjustSystemClock(val *syscall.Timex) (status timex.State, err error) {
	suite.clockLock.Lock()
	defer suite.clockLock.Unlock()

	suite.clockAdjustments = append(suite.clockAdjustments, time.Duration(val.Offset)*time.Nanosecond)

	return
}

func (suite *NTPSuite) fakeQuery(host string) (resp *beevikntp.Response, err error) {
	switch host {
	case "127.0.0.1": // error
		return nil, fmt.Errorf("no response")
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
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	case "127.0.0.4": // adjust +2ms
		resp = &beevikntp.Response{
			Stratum:       1,
			Time:          suite.systemClock,
			ReferenceTime: suite.systemClock,
			ClockOffset:   2 * time.Millisecond,
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	case "127.0.0.5": // adjust +2*epoch
		resp = &beevikntp.Response{
			Stratum:       1,
			Time:          suite.systemClock,
			ReferenceTime: suite.systemClock,
			ClockOffset:   ntp.EpochLimit * 2,
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	case "127.0.0.6": // adjust 1ms/fail alternating
		suite.failingServer++

		if suite.failingServer%2 == 0 {
			return nil, fmt.Errorf("failed this time")
		}

		resp = &beevikntp.Response{
			Stratum:       1,
			Time:          suite.systemClock,
			ReferenceTime: suite.systemClock,
			ClockOffset:   time.Millisecond,
		}

		suite.Require().NoError(resp.Validate())

		return resp, nil
	default:
		return nil, fmt.Errorf("unknown host %q", host)
	}
}

func (suite *NTPSuite) TestSync() {
	syncer := ntp.NewSyncer(log.New(log.Writer(), "ntp ", log.LstdFlags), []string{constants.DefaultNTPServer})

	syncer.SetTime = suite.setSystemClock
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
	syncer := ntp.NewSyncer(log.New(log.Writer(), "ntp ", log.LstdFlags), []string{"127.0.0.3"})

	syncer.SetTime = suite.setSystemClock
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
				return retry.ExpectedError(fmt.Errorf("not enough syncs"))
			}

			return nil
		}))

	cancel()

	wg.Wait()
}

func (suite *NTPSuite) TestSyncChangeTimeservers() {
	syncer := ntp.NewSyncer(log.New(log.Writer(), "ntp ", log.LstdFlags), []string{"127.0.0.1"})

	syncer.SetTime = suite.setSystemClock
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
	syncer := ntp.NewSyncer(log.New(log.Writer(), "ntp ", log.LstdFlags), []string{"127.0.0.1", "127.0.0.2", "127.0.0.3", "127.0.0.4"})

	syncer.SetTime = suite.setSystemClock
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
				return retry.ExpectedError(fmt.Errorf("not enough syncs"))
			}

			return nil
		}))

	cancel()

	wg.Wait()

	// should always sync with 127.0.0.3, and never switch to 127.0.0.4
	for i := 0; i < 3; i++ {
		suite.Assert().Equal(time.Millisecond, suite.clockAdjustments[i])
	}
}

func (suite *NTPSuite) TestSyncEpochChange() {
	syncer := ntp.NewSyncer(log.New(log.Writer(), "ntp ", log.LstdFlags), []string{"127.0.0.5"})

	syncer.SetTime = suite.setSystemClock
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
	syncer := ntp.NewSyncer(log.New(log.Writer(), "ntp ", log.LstdFlags), []string{"127.0.0.6", "127.0.0.4"})

	syncer.SetTime = suite.setSystemClock
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
				return retry.ExpectedError(fmt.Errorf("not enough syncs"))
			}

			return nil
		}))

	cancel()

	wg.Wait()

	// should start sync with 127.0.0.6, then switch to 127.0.0.4
	suite.Assert().Equal(time.Millisecond, suite.clockAdjustments[0])

	for i := 1; i < 3; i++ {
		suite.Assert().Equal(2*time.Millisecond, suite.clockAdjustments[i])
	}
}
