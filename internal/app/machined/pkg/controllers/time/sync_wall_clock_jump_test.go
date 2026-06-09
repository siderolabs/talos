// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time_test

import (
	"context"
	"testing"
	stdtime "time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/time"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

type SyncWallClockJumpSuite struct {
	ctest.DefaultSuite

	detector *fakeClockJumpDetector
	syncer   *noopSyncer
}

func (suite *SyncWallClockJumpSuite) TestDetectedJumpIncrementsEpochWhenSyncDisabled() {
	suite.registerSyncControllerWithMode(runtime.ModeContainer)
	suite.createDefaultTimeServers()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        0,
		SyncDisabled: true,
	})

	suite.detector.enqueueJump()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        1,
		SyncDisabled: true,
	})
}

func (suite *SyncWallClockJumpSuite) TestDetectedJumpIsRetainedUntilInputsAreAvailable() {
	suite.registerSyncControllerWithMode(runtime.ModeContainer)
	timeServers := suite.createDefaultTimeServers()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        0,
		SyncDisabled: true,
	})

	suite.Destroy(timeServers)

	suite.detector.enqueueJump()

	suite.createDefaultTimeServers()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        1,
		SyncDisabled: true,
	})
}

func (suite *SyncWallClockJumpSuite) TestDetectorNoEpochChange() {
	suite.registerSyncControllerWithMode(runtime.ModeMetal)
	suite.createDefaultTimeServers()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       false,
		Epoch:        0,
		SyncDisabled: false,
	})

	suite.syncer.sendSynced()
	suite.detector.enqueueJump()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        0,
		SyncDisabled: false,
	})
}

func (suite *SyncWallClockJumpSuite) registerSyncControllerWithMode(mode runtime.Mode) {
	suite.detector = newFakeClockJumpDetector()
	suite.syncer = newNoopSyncer()

	suite.Require().NoError(suite.Runtime().RegisterController(&time.SyncController{
		V1Alpha1Mode: mode,
		NewNTPSyncer: func(*zap.Logger, []string, bool) time.NTPSyncer {
			return suite.syncer
		},
		NewClockJumpDetector: func(stdtime.Duration, stdtime.Duration) time.ClockJumpDetector {
			return suite.detector
		},
	}))
}

func (suite *SyncWallClockJumpSuite) createDefaultTimeServers() *network.TimeServerStatus {
	timeServers := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeServers.TypedSpec().NTPServers = []string{constants.DefaultNTPServer}

	suite.Create(timeServers)

	return timeServers
}

func (suite *SyncWallClockJumpSuite) assertTimeStatus(spec timeresource.StatusSpec) {
	ctest.AssertResource(suite, timeresource.StatusID, func(r *timeresource.Status, asrt *assert.Assertions) {
		asrt.Equal(spec, *r.TypedSpec())
	})
}

func TestSyncWallClockJumpSuite(t *testing.T) {
	t.Parallel()

	testSuite := &SyncWallClockJumpSuite{}
	testSuite.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * stdtime.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			deviceStatus := runtimeres.NewDevicesStatus(runtimeres.NamespaceName, runtimeres.DevicesID)
			deviceStatus.TypedSpec().Ready = true
			s.Create(deviceStatus)
		},
	}

	suite.Run(t, testSuite)
}

type fakeClockJumpDetector struct {
	jumps chan struct{}
}

func newFakeClockJumpDetector() *fakeClockJumpDetector {
	return &fakeClockJumpDetector{
		jumps: make(chan struct{}),
	}
}

func (detector *fakeClockJumpDetector) Run(context.Context) <-chan struct{} {
	return detector.jumps
}

func (detector *fakeClockJumpDetector) enqueueJump() {
	detector.jumps <- struct{}{}
}

type noopSyncer struct {
	syncedCh chan struct{}
	epochCh  chan struct{}
}

func newNoopSyncer() *noopSyncer {
	return &noopSyncer{
		syncedCh: make(chan struct{}),
		epochCh:  make(chan struct{}),
	}
}

func (syncer *noopSyncer) Run(ctx context.Context) {
	<-ctx.Done()
}

func (syncer *noopSyncer) Synced() <-chan struct{} {
	return syncer.syncedCh
}

func (syncer *noopSyncer) sendSynced() {
	syncer.syncedCh <- struct{}{}
}

func (syncer *noopSyncer) EpochChange() <-chan struct{} {
	return syncer.epochCh
}

func (syncer *noopSyncer) SetTimeServers([]string) {}
