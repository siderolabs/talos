// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package health_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
)

type CheckSuite struct {
	suite.Suite
}

func (suite *CheckSuite) TestHealthy() {
	settings := health.Settings{
		InitialDelay: time.Millisecond,
		Period:       10 * time.Millisecond,
		Timeout:      time.Millisecond,
	}

	var called uint32

	//nolint:unparam
	check := func(context.Context) error {
		atomic.AddUint32(&called, 1)

		return nil
	}

	var state health.State

	errCh := make(chan error)
	ctx, ctxCancel := context.WithCancel(context.Background())

	go func() {
		errCh <- health.Run(ctx, &settings, &state, check)
	}()

	for range 20 {
		time.Sleep(10 * time.Millisecond)

		if atomic.LoadUint32(&called) > 2 {
			break
		}
	}

	ctxCancel()

	suite.Assert().EqualError(<-errCh, context.Canceled.Error())
	suite.Assert().True(called > 2)

	protoHealth := state.AsProto()
	suite.Assert().False(protoHealth.Unknown)
	suite.Assert().True(protoHealth.Healthy)
	suite.Assert().Equal("", protoHealth.LastMessage)
}

func (suite *CheckSuite) TestHealthChange() {
	settings := health.Settings{
		InitialDelay: time.Millisecond,
		Period:       time.Millisecond,
		Timeout:      time.Millisecond,
	}

	var healthy atomic.Uint32

	check := func(context.Context) error {
		if healthy.Load() == 1 {
			return nil
		}

		return errors.New("health failed")
	}

	var state health.State

	notifyCh := make(chan health.StateChange, 2)
	state.Subscribe(notifyCh)

	errCh := make(chan error)
	ctx, ctxCancel := context.WithCancel(context.Background())

	go func() {
		errCh <- health.Run(ctx, &settings, &state, check)
	}()

	// wait for the first health change
	for range 20 {
		if state.Get().Healthy != nil {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	suite.Require().False(*state.Get().Healthy)
	suite.Require().Equal("health failed", state.Get().LastMessage)

	healthy.Store(1)

	for range 10 {
		time.Sleep(20 * time.Millisecond)

		if *state.Get().Healthy {
			break
		}
	}

	suite.Require().True(*state.Get().Healthy)
	suite.Require().Equal("", state.Get().LastMessage)

	ctxCancel()

	suite.Assert().EqualError(<-errCh, context.Canceled.Error())

	state.Unsubscribe(notifyCh)

	close(notifyCh)

	change := <-notifyCh
	suite.Assert().Nil(change.Old.Healthy)
	suite.Assert().False(*change.New.Healthy)

	change = <-notifyCh
	suite.Assert().False(*change.Old.Healthy)
	suite.Assert().True(*change.New.Healthy)
}

func (suite *CheckSuite) TestCheckAbort() {
	settings := health.Settings{
		InitialDelay: time.Millisecond,
		Period:       time.Millisecond,
		Timeout:      time.Millisecond,
	}

	check := func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Second):
			// should never be triggered, as Timeout is 1ms
			return nil
		}
	}

	var state health.State

	errCh := make(chan error)
	ctx, ctxCancel := context.WithCancel(context.Background())

	go func() {
		errCh <- health.Run(ctx, &settings, &state, check)
	}()

	// wait for the first health change
	for range 20 {
		if state.Get().Healthy != nil {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	suite.Require().False(*state.Get().Healthy)
	suite.Require().Equal("context deadline exceeded", state.Get().LastMessage)

	ctxCancel()

	suite.Assert().EqualError(<-errCh, context.Canceled.Error())
}

func (suite *CheckSuite) TestInitialState() {
	settings := health.Settings{
		InitialDelay: 5 * time.Minute,
	}

	var state health.State

	ctx, ctxCancel := context.WithCancel(context.Background())

	errCh := make(chan error)

	go func() {
		errCh <- health.Run(ctx, &settings, &state, nil)
	}()

	time.Sleep(100 * time.Millisecond)

	suite.Require().Nil(state.Get().Healthy)
	suite.Require().Equal("Unknown", state.Get().LastMessage)

	ctxCancel()

	suite.Assert().EqualError(<-errCh, context.Canceled.Error())
}

func (suite *CheckSuite) TestStartupSilentFailures() {
	settings := health.Settings{
		InitialDelay: time.Millisecond,
		Period:       time.Minute,
		Timeout:      time.Second,

		StartupWindow:  time.Minute,
		StartupPeriod:  5 * time.Millisecond,
		StartupTimeout: time.Second,
	}

	var called atomic.Uint32

	check := func(context.Context) error {
		if called.Add(1) <= 3 {
			return errors.New("not ready yet")
		}

		return nil
	}

	var state health.State

	notifyCh := make(chan health.StateChange, 10)
	state.Subscribe(notifyCh)

	errCh := make(chan error)
	ctx, ctxCancel := context.WithCancel(context.Background())

	go func() {
		errCh <- health.Run(ctx, &settings, &state, check)
	}()

	select {
	case change := <-notifyCh:
		// failures during the startup phase are not reported, so the first change is straight to healthy
		suite.Assert().Nil(change.Old.Healthy)
		suite.Require().NotNil(change.New.Healthy)
		suite.Assert().True(*change.New.Healthy)
	case <-time.After(10 * time.Second):
		suite.FailNow("timed out waiting for health change")
	}

	// no extra check right after the startup phase, the next one happens after the regular period
	time.Sleep(50 * time.Millisecond)
	suite.Assert().EqualValues(4, called.Load())

	ctxCancel()

	suite.Assert().EqualError(<-errCh, context.Canceled.Error())

	state.Unsubscribe(notifyCh)
	suite.Assert().Empty(notifyCh)
}

func (suite *CheckSuite) TestStartupWindowExpired() {
	settings := health.Settings{
		InitialDelay: time.Millisecond,
		Period:       time.Minute,
		Timeout:      time.Second,

		StartupWindow:  100 * time.Millisecond,
		StartupPeriod:  5 * time.Millisecond,
		StartupTimeout: time.Second,
	}

	var called atomic.Uint32

	check := func(context.Context) error {
		called.Add(1)

		return errors.New("health failed")
	}

	var state health.State

	notifyCh := make(chan health.StateChange, 10)
	state.Subscribe(notifyCh)

	errCh := make(chan error)
	ctx, ctxCancel := context.WithCancel(context.Background())

	go func() {
		errCh <- health.Run(ctx, &settings, &state, check)
	}()

	select {
	case change := <-notifyCh:
		// the failure is reported once the startup window expires (without waiting for the regular period)
		suite.Assert().Nil(change.Old.Healthy)
		suite.Require().NotNil(change.New.Healthy)
		suite.Assert().False(*change.New.Healthy)
		suite.Assert().Equal("health failed", change.New.LastMessage)
	case <-time.After(10 * time.Second):
		suite.FailNow("timed out waiting for health change")
	}

	// the check was retried silently during the startup window
	suite.Assert().Greater(called.Load(), uint32(2))

	ctxCancel()

	suite.Assert().EqualError(<-errCh, context.Canceled.Error())
}

func TestCheckSuite(t *testing.T) {
	suite.Run(t, new(CheckSuite))
}
