// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
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
}

func (t *trimTracker) trim(target string) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.trimmed = append(t.trimmed, target)

	return 1024, nil
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
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.VolumeTrimController{
					TrimFunc: tracker.trim,
				}))
			},
		},
	})
}

// SetupTest resets the shared trim tracker before each test runs.
func (suite *VolumeTrimSuite) SetupTest() {
	suite.tracker.reset()
	suite.DefaultSuite.SetupTest()
}

func (suite *VolumeTrimSuite) createSchedule(id string) {
	schedule := block.NewVolumeTrimSchedule(block.NamespaceName, id)
	schedule.TypedSpec().Filesystem = block.FilesystemTypeXFS
	schedule.TypedSpec().Interval = runnerTrimInterval
	schedule.TypedSpec().NextTrim = block.NextTrimTime(id, runnerTrimInterval, time.Now())
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

func (suite *VolumeTrimSuite) TestSkipNotMounted() {
	// schedule without a corresponding mount status: trim must be skipped.
	suite.createSchedule("unmounted")

	suite.Assert().Never(func() bool {
		return suite.tracker.count() > 0
	}, 2*runnerTrimInterval, 100*time.Millisecond)

	ctest.AssertNoResource[*block.MountStatus](suite, "unmounted")
}
