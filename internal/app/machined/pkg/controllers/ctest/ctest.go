// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ctest provides basic types and functions for controller testing.
package ctest

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/pkg/logging"
)

// DefaultSuite is a base suite for controller testing.
type DefaultSuite struct { //nolint:govet
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	AfterSetup    func(suite *DefaultSuite)
	AfterTearDown func(suite *DefaultSuite)
}

// SetupTest is a function for setting up a test.
func (suite *DefaultSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.startRuntime()

	if suite.AfterSetup != nil {
		suite.AfterSetup(suite)
	}
}

func (suite *DefaultSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

// Runtime returns the runtime of the suite.
func (suite *DefaultSuite) Runtime() *runtime.Runtime {
	return suite.runtime
}

// State returns the state of the suite.
func (suite *DefaultSuite) State() state.State {
	return suite.state
}

// Ctx returns the context of the suite.
func (suite *DefaultSuite) Ctx() context.Context {
	return suite.ctx
}

// AssertWithin asserts that fn returns within the given duration without an error.
func (suite *DefaultSuite) AssertWithin(d time.Duration, rate time.Duration, fn func() error) {
	retryer := retry.Constant(d, retry.WithUnits(rate))
	suite.Assert().NoError(retryer.Retry(fn))
}

// TearDownTest is a function for tearing down a test.
func (suite *DefaultSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	if suite.AfterTearDown != nil {
		suite.AfterTearDown(suite)
	}
}

// Suite is a type which dectibes the suite type.
type Suite interface {
	T() *testing.T
	Require() *require.Assertions
	State() state.State
	Ctx() context.Context
}

// UpdateWithConflicts is a type safe wrapper around state.UpdateWithConflicts which uses the provided suite.
func UpdateWithConflicts[T resource.Resource](suite Suite, res T, updateFn func(T) error, options ...state.UpdateOption) T { //nolint:ireturn
	suite.T().Helper()
	result, err := safe.StateUpdateWithConflicts(suite.Ctx(), suite.State(), res.Metadata(), updateFn, options...)
	suite.Require().NoError(err)

	return result
}

// GetUsingResource is a type safe wrapper around state.StateGetResource which uses the provided suite.
func GetUsingResource[T resource.Resource](suite Suite, res T, options ...state.GetOption) (T, error) { //nolint:ireturn
	return safe.StateGetResource(suite.Ctx(), suite.State(), res, options...)
}

// Get is a type safe wrapper around state.Get which uses the provided suite.
func Get[T resource.Resource](suite Suite, ptr resource.Pointer, options ...state.GetOption) (T, error) { //nolint:ireturn
	return safe.StateGet[T](suite.Ctx(), suite.State(), ptr, options...)
}
