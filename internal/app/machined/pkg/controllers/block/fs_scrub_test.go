// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type scrubTracker struct {
	mu sync.Mutex

	scrubbed      []string
	inFlight      int
	maxInFlight   int
	scrubDuration time.Duration
	blocking      bool
}

func (t *scrubTracker) scrub(ctx context.Context, _ *zap.Logger, target string) error {
	t.mu.Lock()

	t.inFlight++
	t.maxInFlight = max(t.maxInFlight, t.inFlight)
	t.scrubbed = append(t.scrubbed, target)

	duration := t.scrubDuration
	blocking := t.blocking

	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		t.inFlight--
		t.mu.Unlock()
	}()

	if blocking {
		<-ctx.Done()

		return ctx.Err()
	}

	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *scrubTracker) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.scrubbed)
}

func (t *scrubTracker) maxConcurrency() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.maxInFlight
}

func (t *scrubTracker) reset(scrubDuration time.Duration, blocking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.scrubbed = nil
	t.inFlight = 0
	t.maxInFlight = 0
	t.scrubDuration = scrubDuration
	t.blocking = blocking
}

type FSScrubSuite struct {
	ctest.DefaultSuite

	tracker *scrubTracker
}

// runnerScrubInterval is kept short so the next scrub slot is reached quickly in tests.
const runnerScrubInterval = 2 * time.Second

func TestFSScrubSuite(t *testing.T) {
	t.Parallel()

	tracker := &scrubTracker{}

	suite.Run(t, &FSScrubSuite{
		tracker: tracker,
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.FSScrubController{
					ScrubFunc: tracker.scrub,
				}))
			},
		},
	})
}

// SetupTest resets the shared scrub tracker before each test runs.
func (suite *FSScrubSuite) SetupTest() {
	suite.tracker.reset(0, false)
	suite.DefaultSuite.SetupTest()
}

func (suite *FSScrubSuite) createSchedule(id string) {
	schedule := block.NewFSScrubSchedule(block.NamespaceName, id)
	schedule.TypedSpec().Filesystem = block.FilesystemTypeXFS
	schedule.TypedSpec().Interval = runnerScrubInterval
	schedule.TypedSpec().NextScrub = block.NextScheduledTime(id, runnerScrubInterval, time.Now())
	suite.Create(schedule)
}

func (suite *FSScrubSuite) createMountStatus(id, target string) {
	mountStatus := block.NewMountStatus(block.NamespaceName, id)
	mountStatus.TypedSpec().Spec.VolumeID = id
	mountStatus.TypedSpec().Target = target
	mountStatus.TypedSpec().Filesystem = block.FilesystemTypeXFS
	suite.Create(mountStatus)
}

func (suite *FSScrubSuite) TestScrubMounted() {
	suite.createMountStatus("volume", "/var/mnt/volume")
	suite.createSchedule("volume")

	// the volume is mounted, so it should be scrubbed at the next scheduled slot.
	suite.Assert().Eventually(func() bool {
		return suite.tracker.count() > 0
	}, 10*time.Second, 100*time.Millisecond)

	// the scrub result should be reported in the status.
	ctest.AssertResource(suite, "volume", func(status *block.FSScrubStatus, asrt *assert.Assertions) {
		asrt.Equal("/var/mnt/volume", status.TypedSpec().Mountpoint)
		asrt.Equal(runnerScrubInterval, status.TypedSpec().Interval)
		asrt.Equal("success", status.TypedSpec().Status)
	})

	// once scrubbed, the finalizer should be released.
	ctest.AssertResource(suite, "volume", func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.False(ms.Metadata().Finalizers().Has((&blockctrls.FSScrubController{}).Name()))
	})
}

func (suite *FSScrubSuite) TestSkipNotMounted() {
	// schedule without a corresponding mount status: scrub must be skipped.
	suite.createSchedule("unmounted")

	suite.Assert().Never(func() bool {
		return suite.tracker.count() > 0
	}, 2*runnerScrubInterval, 100*time.Millisecond)

	ctest.AssertNoResource[*block.MountStatus](suite, "unmounted")
}

func (suite *FSScrubSuite) TestScrubSerially() {
	suite.tracker.reset(300*time.Millisecond, false)

	for _, id := range []string{"volume-a", "volume-b", "volume-c"} {
		suite.createMountStatus(id, "/var/mnt/"+id)
		suite.createSchedule(id)
	}

	// all volumes should eventually be scrubbed...
	suite.Assert().Eventually(func() bool {
		return suite.tracker.count() >= 3
	}, 10*time.Second, 100*time.Millisecond)

	// ...but never in parallel.
	suite.Assert().Equal(1, suite.tracker.maxConcurrency())
}

func (suite *FSScrubSuite) TestAbortOnUnmount() {
	// scrubs block until aborted.
	suite.tracker.reset(0, true)

	suite.createMountStatus("volume", "/var/mnt/volume")
	suite.createSchedule("volume")

	// wait for the scrub to start.
	suite.Assert().Eventually(func() bool {
		return suite.tracker.count() > 0
	}, 10*time.Second, 100*time.Millisecond)

	// tear down the mount status: the in-flight scrub should be aborted and the finalizer released.
	mountStatus := block.NewMountStatus(block.NamespaceName, "volume")

	_, err := suite.State().Teardown(suite.Ctx(), mountStatus.Metadata())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, "volume", func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Empty())
	})

	// the aborted scrub should be reported in the status.
	ctest.AssertResource(suite, "volume", func(status *block.FSScrubStatus, asrt *assert.Assertions) {
		asrt.NotEqual("success", status.TypedSpec().Status)
	})

	suite.Destroy(mountStatus)
}
