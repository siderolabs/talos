// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// fakeLastResort is an in-memory MDLastResortBackend recording force-run calls.
type fakeLastResort struct {
	mu sync.Mutex

	inactive []string
	runCalls map[string]struct{}
}

func newFakeLastResort() *fakeLastResort {
	return &fakeLastResort{runCalls: map[string]struct{}{}}
}

func (f *fakeLastResort) InactiveArrays() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.inactive...), nil
}

func (f *fakeLastResort) RunArray(_ context.Context, device string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.runCalls[device] = struct{}{}

	return nil
}

func (f *fakeLastResort) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.inactive = nil
	f.runCalls = map[string]struct{}{}
}

func (f *fakeLastResort) setInactive(devices ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.inactive = devices
}

func (f *fakeLastResort) ranArray(device string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, ok := f.runCalls[device]

	return ok
}

type MDLastResortSuite struct {
	ctest.DefaultSuite

	md *fakeLastResort
}

func (suite *MDLastResortSuite) SetupTest() {
	suite.md.reset()
	suite.DefaultSuite.SetupTest()
}

func (suite *MDLastResortSuite) createUdevd(running, healthy bool) {
	svc := v1alpha1.NewService("udevd")
	svc.TypedSpec().Running = running
	svc.TypedSpec().Healthy = healthy

	suite.Create(svc)
}

func (suite *MDLastResortSuite) eventually(check func() bool) {
	suite.AssertWithin(2*time.Second, 50*time.Millisecond, func() error {
		if check() {
			return nil
		}

		return retry.ExpectedErrorf("last-resort state not yet reached")
	})
}

func (suite *MDLastResortSuite) TestForceRunsInactiveAfterGrace() {
	suite.md.setInactive("/dev/md0")

	suite.createUdevd(true, true)

	suite.eventually(func() bool {
		return suite.md.ranArray("/dev/md0")
	})
}

func (suite *MDLastResortSuite) TestWaitsForUdevdBeforeForcing() {
	suite.md.setInactive("/dev/md0")

	// udevd not yet healthy: grace must not arm, array must stay untouched.
	suite.createUdevd(true, false)

	time.Sleep(400 * time.Millisecond)
	suite.Assert().False(suite.md.ranArray("/dev/md0"))
}

func (suite *MDLastResortSuite) TestNoInactiveArraysIsNoOp() {
	suite.createUdevd(true, true)

	time.Sleep(400 * time.Millisecond)
	suite.Assert().False(suite.md.ranArray("/dev/md0"))
}

func TestMDLastResortSuite(t *testing.T) {
	t.Parallel()

	backend := newFakeLastResort()

	s := &MDLastResortSuite{md: backend}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.MDLastResortController{
				GracePeriod: 100 * time.Millisecond,
				MD:          backend,
			}))
		},
	}

	suite.Run(t, s)
}
