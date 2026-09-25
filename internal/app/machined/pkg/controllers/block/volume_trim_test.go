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

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type trimTracker struct {
	mu      sync.Mutex
	trimmed []string
	options []block.TrimOptionsSpec

	// blockUntilCanceled makes the trim block until the context is canceled (simulating a long trim).
	blockUntilCanceled bool
	started            int
	interrupted        int
}

func (t *trimTracker) trim(ctx context.Context, target string, options block.TrimOptionsSpec) (uint64, error) {
	t.mu.Lock()
	t.started++
	blocking := t.blockUntilCanceled
	t.mu.Unlock()

	if blocking {
		<-ctx.Done()

		t.mu.Lock()
		t.interrupted++
		t.mu.Unlock()

		return 0, ctx.Err()
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.trimmed = append(t.trimmed, target)
	t.options = append(t.options, options)

	return 1024, nil
}

func (t *trimTracker) setBlockUntilCanceled(block bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.blockUntilCanceled = block
}

func (t *trimTracker) startedCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.started
}

func (t *trimTracker) interruptedCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.interrupted
}

func (t *trimTracker) lastOptions() block.TrimOptionsSpec {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.options[len(t.options)-1]
}

func (t *trimTracker) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.trimmed)
}

func (t *trimTracker) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.trimmed = nil
	t.options = nil
	t.blockUntilCanceled = false
	t.started = 0
	t.interrupted = 0
}

type VolumeTrimSuite struct {
	ctest.DefaultSuite

	tracker *trimTracker
}

// runnerTrimInterval is kept short so the next trim slot is reached quickly in tests.
const runnerTrimInterval = 2 * time.Second

func TestVolumeTrimSuite(t *testing.T) {
	t.Parallel()

	tracker := &trimTracker{}

	suite.Run(t, &VolumeTrimSuite{
		tracker: tracker,
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.VolumeTrimController{
				TrimFunc: tracker.trim,
			}))
		},
	})
}

// SetupTest resets the shared trim tracker before each test runs.
func (suite *VolumeTrimSuite) SetupTest() {
	suite.tracker.reset()
	suite.DefaultSuite.SetupTest()
}

func (suite *VolumeTrimSuite) createSchedule(id string) {
	suite.createScheduleWithOptions(id, block.TrimOptionsSpec{})
}

func (suite *VolumeTrimSuite) createScheduleWithOptions(id string, options block.TrimOptionsSpec) {
	schedule := block.NewVolumeTrimSchedule(block.NamespaceName, id)
	schedule.TypedSpec().Filesystem = block.FilesystemTypeXFS
	schedule.TypedSpec().Interval = runnerTrimInterval
	schedule.TypedSpec().NextTrim = block.NextScheduledTime(id, runnerTrimInterval, time.Now())
	schedule.TypedSpec().Options = options
	suite.Create(schedule)
}

func (suite *VolumeTrimSuite) createMountStatus(id, target string) {
	mountStatus := block.NewMountStatus(block.NamespaceName, id)
	mountStatus.TypedSpec().Spec.VolumeID = id
	mountStatus.TypedSpec().Target = target
	mountStatus.TypedSpec().Filesystem = block.FilesystemTypeXFS
	suite.Create(mountStatus)
}

func (suite *VolumeTrimSuite) TestTrimMounted() {
	suite.createMountStatus("volume", "/var/mnt/volume")
	suite.createSchedule("volume")

	// the volume is mounted, so it should be trimmed at the next scheduled slot.
	suite.Assert().Eventually(func() bool {
		return suite.tracker.count() > 0
	}, 10*time.Second, 100*time.Millisecond)

	// once trimmed, the finalizer should be released.
	ctest.AssertResource(suite, "volume", func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.False(ms.Metadata().Finalizers().Has((&blockctrls.VolumeTrimController{}).Name()))
	})
}

func (suite *VolumeTrimSuite) TestTrimOptions() {
	options := block.TrimOptionsSpec{
		ChunkSize:  1024 * 1024 * 1024,
		ChunkDelay: 250 * time.Millisecond,
		MinLength:  1024 * 1024,
	}

	suite.createMountStatus("volume-options", "/var/mnt/volume-options")
	suite.createScheduleWithOptions("volume-options", options)

	suite.Assert().Eventually(func() bool {
		return suite.tracker.count() > 0
	}, 10*time.Second, 100*time.Millisecond)

	// the options from the schedule should be passed to the trim function.
	suite.Assert().Equal(options, suite.tracker.lastOptions())
}

func (suite *VolumeTrimSuite) TestCancelOnUnmount() {
	suite.tracker.setBlockUntilCanceled(true)

	suite.createMountStatus("volume-unmount", "/var/mnt/volume-unmount")
	suite.createSchedule("volume-unmount")

	// wait for the (long-running) trim to start.
	suite.Assert().Eventually(func() bool {
		return suite.tracker.startedCount() > 0
	}, 10*time.Second, 100*time.Millisecond)

	// while trimming, the controller holds a finalizer on the mount status.
	ctest.AssertResource(suite, "volume-unmount", func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Has((&blockctrls.VolumeTrimController{}).Name()))
	})

	// tear down the mount status (the volume is going to be unmounted): the trim should be canceled.
	_, err := suite.State().Teardown(suite.Ctx(), block.NewMountStatus(block.NamespaceName, "volume-unmount").Metadata())
	suite.Require().NoError(err)

	suite.Assert().Eventually(func() bool {
		return suite.tracker.interruptedCount() > 0
	}, 10*time.Second, 100*time.Millisecond)

	// once the trim is canceled, the finalizer should be released so the unmount can proceed.
	ctest.AssertResource(suite, "volume-unmount", func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.False(ms.Metadata().Finalizers().Has((&blockctrls.VolumeTrimController{}).Name()))
	})

	// nothing was actually trimmed.
	suite.Assert().Equal(0, suite.tracker.count())
}

func (suite *VolumeTrimSuite) TestSkipNotMounted() {
	// schedule without a corresponding mount status: trim must be skipped.
	suite.createSchedule("unmounted")

	suite.Assert().Never(func() bool {
		return suite.tracker.count() > 0
	}, 2*runnerTrimInterval, 100*time.Millisecond)

	ctest.AssertNoResource[*block.MountStatus](suite, "unmounted")
}
